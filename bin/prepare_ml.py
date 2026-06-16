from pathlib import Path
import sqlite3
import pandas as pd


DB_PATH = Path(__file__).parent.parent / "data" / "steam_etl.db"
CURRENCY = "USD"
DATA_END = pd.Timestamp.now(tz="UTC").normalize()
OUTPUT_TABLE = "price_logs_gold"
PRICE_DROP_THRESHOLD = 0.05


def process_game(events: pd.DataFrame, data_end: pd.Timestamp) -> pd.DataFrame:
    events = (
        events.sort_values("timestamp")
        .drop_duplicates(subset="timestamp", keep="last")
        .copy()
    )

    start_date = events["timestamp"].min().normalize()

    daily_times = pd.date_range(
        start=start_date,
        end=data_end,
        freq="D",
    ) + pd.Timedelta(hours=23, minutes=59, seconds=59)

    daily = pd.DataFrame({"snapshot_time": daily_times})

    daily = pd.merge_asof(
        daily.sort_values("snapshot_time"),
        events.sort_values("timestamp"),
        left_on="snapshot_time",
        right_on="timestamp",
        direction="backward",
    )

    daily = daily.dropna(subset=["price_cents"]).copy()

    daily["steam_id"] = events["steam_id"].iloc[0]

    daily["price_cents"] = daily["price_cents"].astype(int)
    daily["regular_price_cents"] = daily["regular_price_cents"].astype(int)
    daily["discount_percent"] = daily["discount_percent"].fillna(0).astype(int)

    daily["is_discounted"] = (daily["discount_percent"] > 0).astype(int)

    daily["days_since_price_change"] = (
        (daily["snapshot_time"] - daily["timestamp"]).dt.total_seconds() // 86_400
    ).astype(int)

    daily["min_price_90d"] = (
        daily["price_cents"].rolling(window=90, min_periods=1).min().astype(int)
    )

    daily["price_changes_90d"] = (
        daily["timestamp"]
        .ne(daily["timestamp"].shift())
        .astype(int)
        .rolling(window=90, min_periods=1)
        .sum()
        .astype(int)
    )

    daily["month"] = daily["snapshot_time"].dt.month

    daily["future_min_price_30d"] = (
        daily["price_cents"]
        .shift(-1)
        .iloc[::-1]
        .rolling(window=30, min_periods=30)
        .min()
        .iloc[::-1]
    )

    daily["target_cheaper_next_30d"] = pd.Series(
        pd.NA,
        index=daily.index,
        dtype="Int64",
    )

    has_full_future = daily["future_min_price_30d"].notna()

    daily.loc[
        has_full_future,
        "target_cheaper_next_30d",
    ] = (
        daily.loc[
            has_full_future,
            "future_min_price_30d",
        ]
        <= daily.loc[
            has_full_future,
            "price_cents",
        ]
        * (1 - PRICE_DROP_THRESHOLD)
    ).astype(int)

    weekly = daily[daily["snapshot_time"].dt.dayofweek == 0].copy()

    weekly["snapshot_date"] = weekly["snapshot_time"].dt.date

    return weekly[
        [
            "steam_id",
            "snapshot_date",
            "price_cents",
            "regular_price_cents",
            "discount_percent",
            "is_discounted",
            "days_since_price_change",
            "min_price_90d",
            "price_changes_90d",
            "month",
            "future_min_price_30d",
            "target_cheaper_next_30d",
        ]
    ]


conn = sqlite3.connect(database=DB_PATH)
games = pd.read_sql_query("SELECT * FROM games_gold", conn)
price_logs = pd.read_sql_query("SELECT * FROM price_logs_silver", conn)

# print(games.head())
# print(price_logs.head())

price_logs["timestamp"] = pd.to_datetime(
    price_logs["timestamp"], utc=True, errors="coerce"
)

price_logs = price_logs.rename(
    columns={
        "price_amount_int": "price_cents",
        "regular_amount_int": "regular_price_cents",
        "cut": "discount_percent",
    }
)

price_logs = price_logs[price_logs["price_currency"] == CURRENCY].copy()

numeric_columns = [
    "steam_id",
    "price_cents",
    "regular_price_cents",
    "discount_percent",
]
for column in numeric_columns:
    price_logs[column] = pd.to_numeric(
        price_logs[column],
        errors="coerce",
    )
price_logs = price_logs.dropna(
    subset=[
        "steam_id",
        "timestamp",
        "price_cents",
        "regular_price_cents",
    ]
)

price_logs = price_logs.sort_values(
    ["steam_id", "timestamp", "price_cents"], ascending=[True, True, True]
).drop_duplicates(subset=["steam_id", "timestamp"], keep="first")

price_logs["steam_id"] = price_logs["steam_id"].astype(int)

data_end = DATA_END
price_logs = price_logs[price_logs["timestamp"] < data_end + pd.Timedelta(days=1)]

processed_games = []
for _, price_events in price_logs.groupby("steam_id"):
    processed_games.append(process_game(price_events, data_end))
if not processed_games:
    raise ValueError("No usable price histories were found.")

dataset = pd.concat(
    processed_games,
    ignore_index=True,
)

dataset = dataset.dropna(subset=["target_cheaper_next_30d"])

dataset["target_cheaper_next_30d"] = dataset["target_cheaper_next_30d"].astype(int)

dataset = dataset.sort_values(["snapshot_date", "steam_id"])

dataset.to_sql(
    OUTPUT_TABLE,
    conn,
    if_exists="replace",
    index=False,
)

print(f"Saved to db as {OUTPUT_TABLE}")
