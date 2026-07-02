package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func load(
	games []GameDB,
	prices []PriceDB,
	dates []DatesDB,
	platforms []PlatformsDB,
	priceLogs []PriceLogsDB,
	reviews []ReviewsDB,
) {
	db, err := sql.Open("sqlite", "data/steam_etl.db")
	if err != nil {
		panicf("%s", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			panicf("%s", err)
		}
	}()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		panicf("%s", err)
	}

	if err := createSchema(db); err != nil {
		fmt.Print(scriticalf("%s", err))
		return
	}

	if ctx.full {
		if err := insertGames(db, games); err != nil {
			fmt.Print(scriticalf("%s", err))

			if err := writeToFile(games, "games"); err != nil {
				panicf("%s", err)
			}

			return
		}

		if err := insertPrices(db, prices); err != nil {
			fmt.Print(scriticalf("%s", err))

			if err := writeToFile(prices, "prices"); err != nil {
				panicf("%s", err)
			}
		}

		if err := insertDates(db, dates); err != nil {
			fmt.Print(scriticalf("%s", err))

			if err := writeToFile(dates, "dates"); err != nil {
				panicf("%s", err)
			}
		}

		if err := insertPlatforms(db, platforms); err != nil {
			fmt.Print(scriticalf("%s", err))

			if err := writeToFile(platforms, "platforms"); err != nil {
				panicf("%s", err)
			}
		}

		if err := insertReviews(db, reviews); err != nil {
			fmt.Print(scriticalf("%s", err))

			if err := writeToFile(reviews, "reviews"); err != nil {
				panicf("%s", err)
			}
		}
	}

	if err := insertPriceLogs(db, priceLogs); err != nil {
		fmt.Print(scriticalf("%s", err))

		if err := writeToFile(priceLogs, "priceLogs"); err != nil {
			panicf("%s", err)
		}
	}
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			steam_id INTEGER PRIMARY KEY,
			name TEXT,
			copies_sold INTEGER NOT NULL,
			unreleased BOOLEAN NOT NULL,
			early_access BOOLEAN NOT NULL,
			developers TEXT NOT NULL,
			publishers TEXT NOT NULL,
			publisher_class TEXT NOT NULL,
			genres TEXT NOT NULL,
			categories TEXT NOT NULL,
			type TEXT NOT NULL,
			is_free BOOLEAN NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS prices (
			steam_id INTEGER PRIMARY KEY,
			price_currency TEXT,
			price_initial REAL NOT NULL,
			price_final REAL NOT NULL,
			price_discount INTEGER NOT NULL,

			FOREIGN KEY (steam_id)
				REFERENCES games(steam_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS dates (
			steam_id INTEGER PRIMARY KEY,
			first_release_date TEXT,
			release_date TEXT,
			early_access_exit_date TEXT,
			ea_release_date TEXT,

			FOREIGN KEY (steam_id)
				REFERENCES games(steam_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS platforms (
			steam_id INTEGER PRIMARY KEY,
			platforms_windows BOOLEAN NOT NULL,
			platforms_mac BOOLEAN NOT NULL,
			platforms_linux BOOLEAN NOT NULL,

			FOREIGN KEY (steam_id)
				REFERENCES games(steam_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS price_logs (
			steam_id INTEGER NOT NULL,
			timestamp_utc TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			price_amount REAL NOT NULL,
			price_amount_int INTEGER NOT NULL,
			price_currency TEXT NOT NULL,
			regular_amount REAL NOT NULL,
			regular_amount_int INTEGER NOT NULL,
			regular_currency TEXT NOT NULL,
			cut INTEGER NOT NULL,

			PRIMARY KEY (
				steam_id,
				timestamp_utc,
				price_amount_int
			),

			FOREIGN KEY (steam_id)
				REFERENCES games(steam_id)
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS reviews (
			steam_id INTEGER PRIMARY KEY,
			review_score INTEGER NOT NULL,
			metascore INTEGER,
			user_score REAL,

			FOREIGN KEY (steam_id)
				REFERENCES games(steam_id)
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func insertGames(db *sql.DB, games []GameDB) error {
	stmt, err := db.Prepare(`
		INSERT INTO games (
			steam_id,
			name,
			copies_sold,
			unreleased,
			early_access,
			developers,
			publishers,
			publisher_class,
			genres,
			categories,
			type,
			is_free
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

		ON CONFLICT(steam_id) DO UPDATE SET
			name = excluded.name,
			copies_sold = excluded.copies_sold,
			unreleased = excluded.unreleased,
			early_access = excluded.early_access,
			developers = excluded.developers,
			publishers = excluded.publishers,
			publisher_class = excluded.publisher_class,
			genres = excluded.genres,
			categories = excluded.categories,
			type = excluded.type,
			is_free = excluded.is_free
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, game := range games {
		_, err := stmt.Exec(
			game.SteamId,
			game.Name,
			game.CopiesSold,
			game.Unreleased,
			game.EarlyAccess,
			game.Developers,
			game.Publishers,
			game.PublisherClass,
			game.Genres,
			game.Categories,
			game.Type,
			game.IsFree,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertPrices(db *sql.DB, prices []PriceDB) error {
	stmt, err := db.Prepare(`
		INSERT INTO prices (
			steam_id,
			price_currency,
			price_initial,
			price_final,
			price_discount
		)
		VALUES (?, ?, ?, ?, ?)

		ON CONFLICT(steam_id) DO UPDATE SET
			price_currency = excluded.price_currency,
			price_initial = excluded.price_initial,
			price_final = excluded.price_final,
			price_discount = excluded.price_discount
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, price := range prices {
		_, err := stmt.Exec(
			price.SteamId,
			price.PriceCurrency,
			price.PriceInitial,
			price.PriceFinal,
			price.PriceDiscount,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertDates(db *sql.DB, dates []DatesDB) error {
	stmt, err := db.Prepare(`
		INSERT INTO dates (
			steam_id,
			first_release_date,
			release_date,
			early_access_exit_date,
			ea_release_date
		)
		VALUES (?, ?, ?, ?, ?)

		ON CONFLICT(steam_id) DO UPDATE SET
			first_release_date = excluded.first_release_date,
			release_date = excluded.release_date,
			early_access_exit_date = excluded.early_access_exit_date,
			ea_release_date = excluded.ea_release_date
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, date := range dates {
		_, err := stmt.Exec(
			date.SteamId,
			nullableString(date.FirstReleaseDate),
			nullableString(date.ReleaseDate),
			nullableString(date.EarlyAccessExitDate),
			nullableString(date.EAReleaseDate),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertPlatforms(
	db *sql.DB,
	platforms []PlatformsDB,
) error {
	stmt, err := db.Prepare(`
		INSERT INTO platforms (
			steam_id,
			platforms_windows,
			platforms_mac,
			platforms_linux
		)
		VALUES (?, ?, ?, ?)

		ON CONFLICT(steam_id) DO UPDATE SET
			platforms_windows = excluded.platforms_windows,
			platforms_mac = excluded.platforms_mac,
			platforms_linux = excluded.platforms_linux
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, platform := range platforms {
		_, err := stmt.Exec(
			platform.SteamId,
			platform.PlatformsWindows,
			platform.PlatformsMac,
			platform.PlatformsLinux,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertPriceLogs(
	db *sql.DB,
	priceLogs []PriceLogsDB,
) error {
	stmt, err := db.Prepare(`
		INSERT OR IGNORE INTO price_logs (
			steam_id,
			timestamp_utc,
			timestamp,
			price_amount,
			price_amount_int,
			price_currency,
			regular_amount,
			regular_amount_int,
			regular_currency,
			cut
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, priceLog := range priceLogs {
		_, err := stmt.Exec(
			priceLog.SteamId,
			priceLog.TimestampUTC,
			priceLog.Timestamp,
			priceLog.PriceAmount,
			priceLog.PriceAmountInt,
			priceLog.PriceCurrency,
			priceLog.RegularAmount,
			priceLog.RegularAmountInt,
			priceLog.RegularCurrency,
			priceLog.Cut,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertReviews(
	db *sql.DB,
	reviews []ReviewsDB,
) error {
	stmt, err := db.Prepare(`
		INSERT INTO reviews (
			steam_id,
			review_score,
			metascore,
			user_score
		)
		VALUES (?, ?, ?, ?)

		ON CONFLICT(steam_id) DO UPDATE SET
			review_score = excluded.review_score,
			metascore = excluded.metascore,
			user_score = excluded.user_score
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, review := range reviews {
		_, err := stmt.Exec(
			review.SteamId,
			review.ReviewScore,
			nullableInt(review.Metascore),
			nullableFloat(review.UserScore),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}

	return value
}

func nullableFloat(value float32) any {
	if value == 0 {
		return nil
	}

	return value
}
