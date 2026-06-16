package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func load(games []GameGold, priceLogs []PriceLogSilver) {
	db, err := sql.Open("sqlite", "data/steam_etl.db")
	if err != nil {
		panicf("%s", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			panicf("%s", err)
		}
	}()

	if err := createSchema(db); err != nil {
		msg := scriticalf("%s", err)
		fmt.Print(msg)
	}

	if ctx.full {
		if err := insertGames(db, games); err != nil {
			msg := scriticalf("%s", err)
			fmt.Print(msg)
			if err = writeToFile(games, "games"); err != nil {
				panicf("%s", err)
			}
		}
	}

	if err := insertPriceLogs(db, priceLogs); err != nil {
		msg := scriticalf("%s", err)
		fmt.Print(msg)
		if err = writeToFile(priceLogs, "priceLogs"); err != nil {
			panicf("%s", err)
		}
	}
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS games_gold (
			steam_id INTEGER PRIMARY KEY,
			name TEXT NULL,
			copies_sold INTEGER,
			unreleased BOOLEAN NOT NULL,
			early_access BOOLEAN NOT NULL,
			first_release_date INTEGER NOT NULL,
			release_date INTEGER NOT NULL, 
			early_access_exit_date INTEGER NOT NULL,
			ea_release_date INTEGER NOT NULL,
			developers TEXT NOT NULL,
			publishers TEXT NOT NULL,
			publisher_class TEXT NOT NULL,
			review_score INTEGER NOT NULL,
			genres TEXT NOT NULL,
			categories TEXT NOT NULL,
			type TEXT NOT NULL,
			is_free BOOLEAN NOT NULL,
			price_currency TEXT NOT NULL,
			price_initial INTEGER NOT NULL, 
			price_final INTEGER NOT NULL,
			price_discount INTEGER NOT NULL,
			platforms_windows BOOLEAN,
			platforms_mac BOOLEAN,
			platforms_linux BOOLEAN
		);
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS price_logs_silver (
			steam_id INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			price_amount REAL NOT NULL,
			price_amount_int INTEGER NOT NULL,
			price_currency TEXT NOT NULL,
			regular_amount REAL NOT NULL,
			regular_amount_int INTEGER NOT NULL,
			regular_currency TEXT NOT NULL,
			cut INTEGER,

			PRIMARY KEY (steam_id, timestamp, price_amount_int),
			
			FOREIGN KEY (steam_id)
				REFERENCES games_gold(steam_id)
		);		
	`)
	if err != nil {
		return err
	}

	return nil
}

func insertGames(db *sql.DB, games []GameGold) error {
	stmt, err := db.Prepare(`
		INSERT OR IGNORE INTO games_gold (
			steam_id,
			name,
			copies_sold,
			unreleased,
			early_access,
			first_release_date,
			release_date, 
			early_access_exit_date,
			ea_release_date,
			developers,
			publishers,
			publisher_class,
			review_score,
			genres,
			categories,
			type,
			is_free,
			price_currency,
			price_initial, 
			price_final,
			price_discount,
			platforms_windows,
			platforms_mac,
			platforms_linux
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}

	for _, game := range games {
		_, err := stmt.Exec(
			game.SteamId,
			game.Name,
			game.CopiesSold,
			game.Unreleased,
			game.EarlyAccess,
			game.FirstReleaseDate,
			game.ReleaseDate,
			game.EarlyAccessExitDate,
			game.EAReleaseDate,
			game.Developers,
			game.Publishers,
			game.PublisherClass,
			game.ReviewScore,
			game.Genres,
			game.Categories,
			game.Type,
			game.IsFree,
			game.PriceCurrency,
			game.PriceInitial,
			game.PriceFinal,
			game.PriceDiscount,
			game.PlatformsWindows,
			game.PlatformsMac,
			game.PlatformsLinux,
		)

		if err != nil {
			return err
		}
	}

	return nil
}
func insertPriceLogs(db *sql.DB, priceLogs []PriceLogSilver) error {
	stmt, err := db.Prepare(`
		INSERT OR IGNORE INTO price_logs_silver (
			steam_id,
			timestamp,
			price_amount,
			price_amount_int,
			price_currency,
			regular_amount,
			regular_amount_int,
			regular_currency,
			cut
		)		
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}

	for _, priceLog := range priceLogs {
		_, err := stmt.Exec(
			priceLog.SteamId,
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
