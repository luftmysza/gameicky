import sqlite3
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.dummy import DummyClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import (
    accuracy_score,
    average_precision_score,
    balanced_accuracy_score,
    classification_report,
    confusion_matrix,
    f1_score,
    precision_score,
    recall_score,
    roc_auc_score,
)
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler


DATABASE_FILE = Path(__file__).parent.parent / "data" / "steam_etl.db"
TABLE_NAME = "price_logs_gold"

MODEL_FILE = "steam_price_model.joblib"
TEST_PREDICTIONS_TABLE = "price_predictions_test"

TEST_FRACTION = 0.20

# The target looks 30 days into the future.
# We therefore leave a 30-day gap between training and testing.
TARGET_HORIZON_DAYS = 30

FEATURES = [
    "price_cents",
    "regular_price_cents",
    "discount_percent",
    "is_discounted",
    "days_since_price_change",
    "min_price_90d",
    "price_changes_90d",
    "month",
]

TARGET = "target_cheaper_next_30d"

IDENTIFIER_COLUMNS = [
    "steam_id",
    "snapshot_date",
]


def load_dataset() -> pd.DataFrame:
    selected_columns = IDENTIFIER_COLUMNS + FEATURES + [TARGET]

    query = f"""
        SELECT
            {", ".join(selected_columns)}
        FROM {TABLE_NAME}
    """

    with sqlite3.connect(DATABASE_FILE) as connection:
        dataset = pd.read_sql_query(query, connection)

    return dataset


def validate_dataset(dataset: pd.DataFrame) -> pd.DataFrame:
    required_columns = set(IDENTIFIER_COLUMNS + FEATURES + [TARGET])

    missing_columns = required_columns - set(dataset.columns)

    if missing_columns:
        raise ValueError(f"Missing columns: {sorted(missing_columns)}")

    dataset = dataset.copy()

    dataset["snapshot_date"] = pd.to_datetime(
        dataset["snapshot_date"],
        errors="coerce",
    )

    numeric_columns = FEATURES + [TARGET]

    for column in numeric_columns:
        dataset[column] = pd.to_numeric(
            dataset[column],
            errors="coerce",
        )

    missing_before = len(dataset)

    dataset = dataset.dropna(subset=(IDENTIFIER_COLUMNS + FEATURES + [TARGET]))

    missing_removed = missing_before - len(dataset)

    if missing_removed:
        print(f"Removed {missing_removed:,} rows containing missing or invalid values.")

    dataset["steam_id"] = dataset["steam_id"].astype(int)
    dataset[TARGET] = dataset[TARGET].astype(int)

    # Gold must contain one row per game per snapshot date.
    duplicate_mask = dataset.duplicated(
        subset=["steam_id", "snapshot_date"],
        keep=False,
    )

    if duplicate_mask.any():
        duplicate_rows = dataset.loc[
            duplicate_mask,
            ["steam_id", "snapshot_date"],
        ].sort_values(["steam_id", "snapshot_date"])

        print("Duplicate gold keys:")
        print(duplicate_rows.head(20).to_string(index=False))

        raise ValueError(
            "price_logs_gold contains duplicate (steam_id, snapshot_date) rows."
        )

    target_values = set(dataset[TARGET].unique())

    if not target_values.issubset({0, 1}):
        raise ValueError(f"Target contains unexpected values: {sorted(target_values)}")

    if target_values != {0, 1}:
        raise ValueError("The dataset must contain both target classes 0 and 1.")

    if len(dataset) < 100:
        raise ValueError(
            "The dataset has fewer than 100 usable rows. "
            "That is probably too small for a meaningful evaluation."
        )

    return dataset.sort_values(["snapshot_date", "steam_id"]).reset_index(drop=True)


def chronological_split(
    dataset: pd.DataFrame,
) -> tuple[pd.DataFrame, pd.DataFrame, pd.Timestamp]:
    unique_dates = np.array(sorted(dataset["snapshot_date"].unique()))

    if len(unique_dates) < 10:
        raise ValueError(
            "Not enough unique snapshot dates for chronological splitting."
        )

    split_position = int(len(unique_dates) * (1 - TEST_FRACTION))

    split_position = min(
        max(split_position, 1),
        len(unique_dates) - 1,
    )

    test_start = pd.Timestamp(unique_dates[split_position])

    # A training row's target uses the following 30 days.
    # Exclude rows whose target window overlaps the test period.
    training_cutoff = test_start - pd.Timedelta(days=TARGET_HORIZON_DAYS)

    train = dataset[dataset["snapshot_date"] < training_cutoff].copy()

    test = dataset[dataset["snapshot_date"] >= test_start].copy()

    if train.empty:
        raise ValueError("Training set is empty after applying the 30-day gap.")

    if test.empty:
        raise ValueError("Testing set is empty.")

    for name, part in [
        ("training", train),
        ("testing", test),
    ]:
        if part[TARGET].nunique() < 2:
            raise ValueError(f"The {name} set does not contain both classes.")

    return train, test, test_start


def positive_probabilities(
    model,
    features: pd.DataFrame,
) -> np.ndarray:
    probabilities = model.predict_proba(features)

    classes = list(model.classes_)

    if 1 not in classes:
        raise ValueError("The fitted model does not contain positive class 1.")

    positive_index = classes.index(1)

    return probabilities[:, positive_index]


def evaluate_model(
    name: str,
    model,
    X_test: pd.DataFrame,
    y_test: pd.Series,
) -> dict:
    predicted_class = model.predict(X_test)

    predicted_probability = positive_probabilities(
        model,
        X_test,
    )

    print()
    print("=" * 60)
    print(name)
    print("=" * 60)

    print("Confusion matrix:")
    print(
        confusion_matrix(
            y_test,
            predicted_class,
            labels=[0, 1],
        )
    )

    print()
    print("Classification report:")
    print(
        classification_report(
            y_test,
            predicted_class,
            labels=[0, 1],
            target_names=[
                "not cheaper",
                "cheaper",
            ],
            digits=3,
            zero_division=0,
        )
    )

    metrics = {
        "model": name,
        "accuracy": accuracy_score(
            y_test,
            predicted_class,
        ),
        "balanced_accuracy": balanced_accuracy_score(
            y_test,
            predicted_class,
        ),
        "precision": precision_score(
            y_test,
            predicted_class,
            zero_division=0,
        ),
        "recall": recall_score(
            y_test,
            predicted_class,
            zero_division=0,
        ),
        "f1": f1_score(
            y_test,
            predicted_class,
            zero_division=0,
        ),
        "average_precision": average_precision_score(
            y_test,
            predicted_probability,
        ),
        "roc_auc": roc_auc_score(
            y_test,
            predicted_probability,
        ),
    }

    print("Summary metrics:")

    for metric, value in metrics.items():
        if metric != "model":
            print(f"  {metric:20s}: {value:.3f}")

    metrics["predicted_class"] = predicted_class
    metrics["predicted_probability"] = predicted_probability

    return metrics


def print_logistic_coefficients(
    model: Pipeline,
) -> None:
    classifier = model.named_steps["classifier"]
    scaler = model.named_steps["scaler"]

    # Coefficients refer to standardized features.
    coefficients = pd.DataFrame(
        {
            "feature": FEATURES,
            "coefficient": classifier.coef_[0],
        }
    )

    coefficients["absolute_coefficient"] = coefficients["coefficient"].abs()

    coefficients = coefficients.sort_values(
        "absolute_coefficient",
        ascending=False,
    )

    print()
    print("Logistic-regression coefficients:")
    print(
        coefficients[["feature", "coefficient"]].to_string(
            index=False,
            float_format=lambda value: f"{value:.4f}",
        )
    )

    print()
    print(
        "Positive coefficient: associated with a greater "
        "probability of becoming cheaper."
    )
    print(
        "Negative coefficient: associated with a lower probability of becoming cheaper."
    )


def save_model(
    model: Pipeline,
    test_start: pd.Timestamp,
    training_rows: int,
) -> None:
    artifact = {
        "model": model,
        "features": FEATURES,
        "target": TARGET,
        "test_start": test_start.isoformat(),
        "training_rows": training_rows,
        "target_horizon_days": TARGET_HORIZON_DAYS,
    }

    joblib.dump(artifact, MODEL_FILE)

    print()
    print(f"Saved trained model to {MODEL_FILE}")


def save_test_predictions(
    test: pd.DataFrame,
    predicted_class: np.ndarray,
    predicted_probability: np.ndarray,
) -> None:
    predictions = test[
        [
            "steam_id",
            "snapshot_date",
            TARGET,
        ]
    ].copy()

    predictions = predictions.rename(
        columns={
            TARGET: "actual_class",
        }
    )

    predictions["predicted_class"] = predicted_class
    predictions["probability_cheaper"] = predicted_probability

    predictions["snapshot_date"] = predictions["snapshot_date"].dt.strftime("%Y-%m-%d")

    with sqlite3.connect(DATABASE_FILE) as connection:
        predictions.to_sql(
            TEST_PREDICTIONS_TABLE,
            connection,
            if_exists="replace",
            index=False,
        )

    print(
        f"Saved {len(predictions):,} test predictions to "
        f"SQLite table {TEST_PREDICTIONS_TABLE}"
    )


def main() -> None:
    if not Path(DATABASE_FILE).exists():
        raise FileNotFoundError(f"SQLite database not found: {DATABASE_FILE}")

    dataset = load_dataset()
    dataset = validate_dataset(dataset)

    train, test, test_start = chronological_split(dataset)

    X_train = train[FEATURES]
    y_train = train[TARGET]

    X_test = test[FEATURES]
    y_test = test[TARGET]

    print("Dataset summary")
    print("-" * 60)
    print(f"Total rows:       {len(dataset):,}")
    print(f"Unique games:     {dataset['steam_id'].nunique():,}")
    print(f"Training rows:    {len(train):,}")
    print(f"Testing rows:     {len(test):,}")
    print(f"Test starts:      {test_start.date()}")
    print(
        f"Training range:   "
        f"{train['snapshot_date'].min().date()} to "
        f"{train['snapshot_date'].max().date()}"
    )
    print(
        f"Testing range:    "
        f"{test['snapshot_date'].min().date()} to "
        f"{test['snapshot_date'].max().date()}"
    )

    print()
    print("Target rates")
    print("-" * 60)
    print(f"Training positive rate: {y_train.mean():.3f}")
    print(f"Testing positive rate:  {y_test.mean():.3f}")

    # Baseline: always predicts the most frequent training class.
    baseline = DummyClassifier(
        strategy="most_frequent",
    )

    baseline.fit(X_train, y_train)

    baseline_metrics = evaluate_model(
        "Most-frequent baseline",
        baseline,
        X_test,
        y_test,
    )

    # Actual model.
    logistic_model = Pipeline(
        steps=[
            (
                "scaler",
                StandardScaler(),
            ),
            (
                "classifier",
                LogisticRegression(
                    max_iter=2_000,
                    class_weight="balanced",
                    solver="lbfgs",
                    random_state=42,
                ),
            ),
        ]
    )

    logistic_model.fit(X_train, y_train)

    logistic_metrics = evaluate_model(
        "Logistic regression",
        logistic_model,
        X_test,
        y_test,
    )

    comparison = pd.DataFrame(
        [
            {
                key: value
                for key, value in baseline_metrics.items()
                if key
                not in {
                    "predicted_class",
                    "predicted_probability",
                }
            },
            {
                key: value
                for key, value in logistic_metrics.items()
                if key
                not in {
                    "predicted_class",
                    "predicted_probability",
                }
            },
        ]
    )

    print()
    print("=" * 60)
    print("Model comparison")
    print("=" * 60)
    print(
        comparison.to_string(
            index=False,
            float_format=lambda value: f"{value:.3f}",
        )
    )

    print_logistic_coefficients(logistic_model)

    save_model(
        logistic_model,
        test_start=test_start,
        training_rows=len(train),
    )

    save_test_predictions(
        test,
        logistic_metrics["predicted_class"],
        logistic_metrics["predicted_probability"],
    )


if __name__ == "__main__":
    main()
