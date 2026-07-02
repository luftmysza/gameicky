from pathlib import Path
import sqlite3

import pandas as pd


DB_PATH = Path(__file__).parent.parent / "data" / "steam_etl.db"

CURRENCY = "USD"

OUTPUT_TABLE = "price_logs_ml"

PRICE_DROP_THRESHOLD = 0.05


def process_game(
    events: pd.DataFrame,
    data_end: pd.Timestamp,
) -> pd.DataFrame:
    events = (
        events.sort_values("timestamp_utc")
        .drop_duplicates(
            subset="timestamp_utc",
            keep="last",
        )
        .copy()
    )

    if events.empty:
        return pd.DataFrame()

    start_date = events["timestamp_utc"].min().normalize()

    daily_times = pd.date_range(
        start=start_date,
        end=data_end,
        freq="D",
        tz="UTC",
    ) + pd.Timedelta(
        hours=23,
        minutes=59,
        seconds=59,
    )

    daily = pd.DataFrame(
        {
            "snapshot_time": daily_times,
        }
    )

    daily = pd.merge_asof(
        daily.sort_values("snapshot_time"),
        events.sort_values("timestamp_utc"),
        left_on="snapshot_time",
        right_on="timestamp_utc",
        direction="backward",
    )

    daily = daily.dropna(
        subset=[
            "price_cents",
            "regular_price_cents",
        ]
    ).copy()

    if daily.empty:
        return pd.DataFrame()

    daily["steam_id"] = int(events["steam_id"].iloc[0])

    daily["price_cents"] = daily["price_cents"].astype(int)

    daily["regular_price_cents"] = daily["regular_price_cents"].astype(int)

    daily["discount_percent"] = daily["discount_percent"].fillna(0).astype(int)

    daily["is_discounted"] = (daily["discount_percent"] > 0).astype(int)

    daily["days_since_price_change"] = (
        (daily["snapshot_time"] - daily["timestamp_utc"]).dt.total_seconds() // 86_400
    ).astype(int)

    daily["min_price_90d"] = (
        daily["price_cents"]
        .rolling(
            window=90,
            min_periods=1,
        )
        .min()
        .astype(int)
    )

    daily["price_changes_90d"] = (
        daily["timestamp_utc"]
        .ne(daily["timestamp_utc"].shift())
        .astype(int)
        .rolling(
            window=90,
            min_periods=1,
        )
        .sum()
        .astype(int)
    )

    daily["month"] = daily["snapshot_time"].dt.month

    daily["year"] = daily["snapshot_time"].dt.year

    daily["current_price_ratio"] = daily["price_cents"] / daily[
        "regular_price_cents"
    ].replace(0, pd.NA)

    daily["future_min_price_30d"] = (
        daily["price_cents"]
        .shift(-1)
        .iloc[::-1]
        .rolling(
            window=30,
            min_periods=30,
        )
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
        <= (
            daily.loc[
                has_full_future,
                "price_cents",
            ]
            * (1 - PRICE_DROP_THRESHOLD)
        )
    ).astype(int)

    weekly = daily[daily["snapshot_time"].dt.dayofweek == 0].copy()

    weekly["snapshot_date"] = weekly["snapshot_time"].dt.strftime("%Y-%m-%d")

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
            "current_price_ratio",
            "month",
            "year",
            "future_min_price_30d",
            "target_cheaper_next_30d",
        ]
    ]


def main() -> None:
    connection = sqlite3.connect(DB_PATH)

    try:
        games = pd.read_sql_query(
            """
            SELECT
                steam_id,
                name,
                copies_sold,
                unreleased,
                early_access,
                publisher_class,
                genres,
                categories,
                type,
                is_free
            FROM games
            """,
            connection,
        )

        price_logs = pd.read_sql_query(
            """
            SELECT
                steam_id,
                timestamp_utc,
                price_amount_int,
                price_currency,
                regular_amount_int,
                regular_currency,
                cut
            FROM price_logs
            """,
            connection,
        )

        if price_logs.empty:
            raise ValueError("The price_logs table contains no records.")

        price_logs["timestamp_utc"] = pd.to_datetime(
            price_logs["timestamp_utc"],
            utc=True,
            errors="coerce",
        )

        price_logs = price_logs.rename(
            columns={
                "price_amount_int": "price_cents",
                "regular_amount_int": "regular_price_cents",
                "cut": "discount_percent",
            }
        )

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
                "timestamp_utc",
                "price_cents",
                "regular_price_cents",
            ]
        ).copy()

        price_logs = price_logs[price_logs["price_currency"] == CURRENCY].copy()

        price_logs = price_logs[price_logs["regular_currency"] == CURRENCY].copy()

        price_logs["steam_id"] = price_logs["steam_id"].astype(int)

        price_logs["price_cents"] = price_logs["price_cents"].astype(int)

        price_logs["regular_price_cents"] = price_logs["regular_price_cents"].astype(
            int
        )

        price_logs["discount_percent"] = (
            price_logs["discount_percent"].fillna(0).astype(int)
        )

        valid_game_ids = set(games["steam_id"].astype(int))

        price_logs = price_logs[price_logs["steam_id"].isin(valid_game_ids)].copy()

        price_logs = price_logs.sort_values(
            [
                "steam_id",
                "timestamp_utc",
                "price_cents",
            ]
        ).drop_duplicates(
            subset=[
                "steam_id",
                "timestamp_utc",
            ],
            keep="last",
        )

        if price_logs.empty:
            raise ValueError("No usable USD price histories were found.")

        now_utc = pd.Timestamp.now(tz="UTC")

        data_end = now_utc.normalize()

        price_logs = price_logs[
            price_logs["timestamp_utc"] < data_end + pd.Timedelta(days=1)
        ].copy()

        processed_games = []

        for steam_id, price_events in price_logs.groupby("steam_id"):
            processed = process_game(
                price_events,
                data_end,
            )

            if not processed.empty:
                processed_games.append(processed)

        if not processed_games:
            raise ValueError("No price histories could be transformed.")

        dataset = pd.concat(
            processed_games,
            ignore_index=True,
        )

        dataset = dataset.dropna(
            subset=[
                "target_cheaper_next_30d",
            ]
        ).copy()

        dataset["future_min_price_30d"] = dataset["future_min_price_30d"].astype(int)

        dataset["target_cheaper_next_30d"] = dataset["target_cheaper_next_30d"].astype(
            int
        )

        dataset = dataset.sort_values(
            [
                "snapshot_date",
                "steam_id",
            ]
        )

        dataset.to_sql(
            OUTPUT_TABLE,
            connection,
            if_exists="replace",
            index=False,
        )

        connection.execute(
            f"""
            CREATE UNIQUE INDEX IF NOT EXISTS
                idx_{OUTPUT_TABLE}_game_date
            ON {OUTPUT_TABLE} (
                steam_id,
                snapshot_date
            )
            """
        )

        connection.execute(
            f"""
            CREATE INDEX IF NOT EXISTS
                idx_{OUTPUT_TABLE}_snapshot_date
            ON {OUTPUT_TABLE} (
                snapshot_date
            )
            """
        )

        connection.commit()

        print(f"Saved {len(dataset)} rows to table {OUTPUT_TABLE}")

        print(f"Games included: {dataset['steam_id'].nunique()}")

        print("Target distribution:")

        print(dataset["target_cheaper_next_30d"].value_counts(dropna=False))

    finally:
        connection.close()


if __name__ == "__main__":
    main()
