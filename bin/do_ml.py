import sqlite3
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.dummy import DummyClassifier
from sklearn.ensemble import RandomForestClassifier
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

TABLE_NAME = "price_logs_ml"

MODEL_FILE = Path(__file__).parent.parent / "data" / "steam_price_model.joblib"

TEST_PREDICTIONS_TABLE = "price_predictions_test"

TEST_FRACTION = 0.20
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

    with sqlite3.connect(DATABASE_FILE) as connection:
        table_exists = connection.execute(
            """
            SELECT 1
            FROM sqlite_master
            WHERE type = 'table'
              AND name = ?
            """,
            (TABLE_NAME,),
        ).fetchone()

        if table_exists is None:
            raise ValueError(
                f"Required table {TABLE_NAME!r} does not exist. "
                "Run the price-log preparation script first."
            )

        query = f"""
            SELECT
                {", ".join(selected_columns)}
            FROM {TABLE_NAME}
        """

        dataset = pd.read_sql_query(
            query,
            connection,
        )

    if dataset.empty:
        raise ValueError(f"The {TABLE_NAME!r} table contains no rows.")

    return dataset


def validate_dataset(
    dataset: pd.DataFrame,
) -> pd.DataFrame:
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

    rows_before = len(dataset)

    dataset = dataset.dropna(subset=(IDENTIFIER_COLUMNS + FEATURES + [TARGET]))

    removed_rows = rows_before - len(dataset)

    if removed_rows:
        print(f"Removed {removed_rows:,} rows containing missing or invalid values.")

    dataset["steam_id"] = dataset["steam_id"].astype(int)

    dataset[TARGET] = dataset[TARGET].astype(int)

    duplicate_mask = dataset.duplicated(
        subset=[
            "steam_id",
            "snapshot_date",
        ],
        keep=False,
    )

    if duplicate_mask.any():
        duplicate_rows = dataset.loc[
            duplicate_mask,
            [
                "steam_id",
                "snapshot_date",
            ],
        ].sort_values(
            [
                "steam_id",
                "snapshot_date",
            ]
        )

        print("Duplicate ML keys:")
        print(
            duplicate_rows.head(20).to_string(
                index=False,
            )
        )

        raise ValueError(
            f"{TABLE_NAME} contains duplicate (steam_id, snapshot_date) rows."
        )

    target_values = set(dataset[TARGET].unique())

    if not target_values.issubset({0, 1}):
        raise ValueError(f"Target contains unexpected values: {sorted(target_values)}")

    if target_values != {0, 1}:
        raise ValueError("The dataset must contain both target classes 0 and 1.")

    if len(dataset) < 100:
        raise ValueError(
            "The dataset has fewer than 100 usable rows. "
            "That is probably too small for meaningful evaluation."
        )

    return dataset.sort_values(
        [
            "snapshot_date",
            "steam_id",
        ]
    ).reset_index(drop=True)


def chronological_split(
    dataset: pd.DataFrame,
) -> tuple[
    pd.DataFrame,
    pd.DataFrame,
    pd.Timestamp,
]:
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

    training_cutoff = test_start - pd.Timedelta(days=TARGET_HORIZON_DAYS)

    train = dataset[dataset["snapshot_date"] < training_cutoff].copy()

    test = dataset[dataset["snapshot_date"] >= test_start].copy()

    if train.empty:
        raise ValueError("Training set is empty after applying the 30-day leakage gap.")

    if test.empty:
        raise ValueError("Testing set is empty.")

    for name, part in [
        ("training", train),
        ("testing", test),
    ]:
        if part[TARGET].nunique() < 2:
            raise ValueError(f"The {name} set does not contain both target classes.")

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
    print("=" * 70)
    print(name)
    print("=" * 70)

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
        "balanced_accuracy": (
            balanced_accuracy_score(
                y_test,
                predicted_class,
            )
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
        "average_precision": (
            average_precision_score(
                y_test,
                predicted_probability,
            )
        ),
        "roc_auc": roc_auc_score(
            y_test,
            predicted_probability,
        ),
        "predicted_positive_count": int(predicted_class.sum()),
    }

    print("Summary metrics:")

    for metric, value in metrics.items():
        if metric == "model":
            continue

        if metric == "predicted_positive_count":
            print(f"  {metric:26s}: {value}")
        else:
            print(f"  {metric:26s}: {value:.3f}")

    metrics["predicted_class"] = predicted_class

    metrics["predicted_probability"] = predicted_probability

    return metrics


def print_logistic_coefficients(
    model: Pipeline,
) -> None:
    classifier = model.named_steps["classifier"]

    coefficients = pd.DataFrame(
        {
            "feature": FEATURES,
            "coefficient": (classifier.coef_[0]),
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
        coefficients[
            [
                "feature",
                "coefficient",
            ]
        ].to_string(
            index=False,
            float_format=(lambda value: f"{value:.4f}"),
        )
    )


def print_random_forest_importances(
    model: RandomForestClassifier,
) -> None:
    importances = pd.DataFrame(
        {
            "feature": FEATURES,
            "importance": (model.feature_importances_),
        }
    )

    importances = importances.sort_values(
        "importance",
        ascending=False,
    )

    print()
    print("Random-Forest feature importances:")

    print(
        importances.to_string(
            index=False,
            float_format=(lambda value: f"{value:.4f}"),
        )
    )


def choose_best_model(
    candidates: list[tuple[object, dict]],
) -> tuple[object, dict]:
    eligible_candidates = [
        candidate
        for candidate in candidates
        if candidate[1]["predicted_positive_count"] > 0
    ]

    if not eligible_candidates:
        raise ValueError(
            "Neither trained model predicted any positive cases. "
            "A most-precise model cannot be selected."
        )

    return max(
        eligible_candidates,
        key=lambda candidate: (
            candidate[1]["precision"],
            candidate[1]["average_precision"],
            candidate[1]["f1"],
            candidate[1]["balanced_accuracy"],
        ),
    )


def save_model(
    model,
    model_metrics: dict,
    test_start: pd.Timestamp,
    training_rows: int,
) -> None:
    artifact = {
        "model": model,
        "model_name": model_metrics["model"],
        "features": FEATURES,
        "target": TARGET,
        "source_table": TABLE_NAME,
        "selection_metric": "precision",
        "precision": model_metrics["precision"],
        "average_precision": (model_metrics["average_precision"]),
        "f1": model_metrics["f1"],
        "balanced_accuracy": (model_metrics["balanced_accuracy"]),
        "test_start": (test_start.isoformat()),
        "training_rows": training_rows,
        "target_horizon_days": (TARGET_HORIZON_DAYS),
    }

    joblib.dump(
        artifact,
        MODEL_FILE,
    )

    print()
    print(f"Saved selected model {model_metrics['model']!r} to {MODEL_FILE}")


def save_test_predictions(
    test: pd.DataFrame,
    model_name: str,
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

    predictions["model_name"] = model_name

    predictions["predicted_class"] = predicted_class.astype(int)

    predictions["probability_cheaper"] = predicted_probability

    predictions["snapshot_date"] = predictions["snapshot_date"].dt.strftime("%Y-%m-%d")

    with sqlite3.connect(DATABASE_FILE) as connection:
        predictions.to_sql(
            TEST_PREDICTIONS_TABLE,
            connection,
            if_exists="replace",
            index=False,
        )

        connection.execute(
            f"""
            CREATE UNIQUE INDEX IF NOT EXISTS
                idx_{TEST_PREDICTIONS_TABLE}_game_date
            ON {TEST_PREDICTIONS_TABLE} (
                steam_id,
                snapshot_date
            )
            """
        )

        connection.execute(
            f"""
            CREATE INDEX IF NOT EXISTS
                idx_{TEST_PREDICTIONS_TABLE}_predicted_class
            ON {TEST_PREDICTIONS_TABLE} (
                predicted_class
            )
            """
        )

        connection.execute(
            f"""
            CREATE INDEX IF NOT EXISTS
                idx_{TEST_PREDICTIONS_TABLE}_probability
            ON {TEST_PREDICTIONS_TABLE} (
                probability_cheaper
            )
            """
        )

        connection.commit()

    print(
        f"Saved {len(predictions):,} predictions "
        f"from {model_name!r} to SQLite table "
        f"{TEST_PREDICTIONS_TABLE}"
    )


def print_dataset_summary(
    dataset: pd.DataFrame,
    train: pd.DataFrame,
    test: pd.DataFrame,
    test_start: pd.Timestamp,
) -> None:
    print("Dataset summary")
    print("-" * 70)

    print(f"Source table:     {TABLE_NAME}")

    print(f"Total rows:       {len(dataset):,}")

    print(f"Unique games:     {dataset['steam_id'].nunique():,}")

    print(f"Training rows:    {len(train):,}")

    print(f"Testing rows:     {len(test):,}")

    print(f"Test starts:      {test_start.date()}")

    print(
        "Training range:   "
        f"{train['snapshot_date'].min().date()} "
        "to "
        f"{train['snapshot_date'].max().date()}"
    )

    print(
        "Testing range:    "
        f"{test['snapshot_date'].min().date()} "
        "to "
        f"{test['snapshot_date'].max().date()}"
    )


def main() -> None:
    if not DATABASE_FILE.exists():
        raise FileNotFoundError(f"SQLite database not found: {DATABASE_FILE}")

    dataset = load_dataset()

    dataset = validate_dataset(dataset)

    train, test, test_start = chronological_split(dataset)

    X_train = train[FEATURES]
    y_train = train[TARGET]

    X_test = test[FEATURES]
    y_test = test[TARGET]

    print_dataset_summary(
        dataset,
        train,
        test,
        test_start,
    )

    print()
    print("Target rates")
    print("-" * 70)

    print(f"Training positive rate: {y_train.mean():.3f}")

    print(f"Testing positive rate:  {y_test.mean():.3f}")

    baseline = DummyClassifier(
        strategy="most_frequent",
    )

    baseline.fit(
        X_train,
        y_train,
    )

    baseline_metrics = evaluate_model(
        "Most-frequent baseline",
        baseline,
        X_test,
        y_test,
    )

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

    logistic_model.fit(
        X_train,
        y_train,
    )

    logistic_metrics = evaluate_model(
        "Logistic regression",
        logistic_model,
        X_test,
        y_test,
    )

    random_forest_model = RandomForestClassifier(
        n_estimators=500,
        max_depth=14,
        min_samples_split=20,
        min_samples_leaf=10,
        max_features="sqrt",
        class_weight="balanced_subsample",
        n_jobs=-1,
        random_state=42,
    )

    random_forest_model.fit(
        X_train,
        y_train,
    )

    random_forest_metrics = evaluate_model(
        "Random Forest",
        random_forest_model,
        X_test,
        y_test,
    )

    comparison_rows = []

    for metrics in [
        baseline_metrics,
        logistic_metrics,
        random_forest_metrics,
    ]:
        comparison_rows.append(
            {
                key: value
                for key, value in metrics.items()
                if key
                not in {
                    "predicted_class",
                    "predicted_probability",
                }
            }
        )

    comparison = pd.DataFrame(comparison_rows)

    print()
    print("=" * 70)
    print("Model comparison")
    print("=" * 70)

    print(
        comparison.to_string(
            index=False,
            float_format=(lambda value: f"{value:.3f}"),
        )
    )

    print_logistic_coefficients(logistic_model)

    print_random_forest_importances(random_forest_model)

    selected_model, selected_metrics = choose_best_model(
        [
            (
                logistic_model,
                logistic_metrics,
            ),
            (
                random_forest_model,
                random_forest_metrics,
            ),
        ]
    )

    print()
    print("=" * 70)
    print("Selected model")
    print("=" * 70)

    print(f"Model:             {selected_metrics['model']}")

    print(f"Precision:         {selected_metrics['precision']:.3f}")

    print(f"Average precision: {selected_metrics['average_precision']:.3f}")

    print(f"F1 score:          {selected_metrics['f1']:.3f}")

    print(f"Predicted positives: {selected_metrics['predicted_positive_count']}")

    save_model(
        selected_model,
        selected_metrics,
        test_start=test_start,
        training_rows=len(train),
    )

    save_test_predictions(
        test,
        selected_metrics["model"],
        selected_metrics["predicted_class"],
        selected_metrics["predicted_probability"],
    )


if __name__ == "__main__":
    main()
