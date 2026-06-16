package main

import (
	"strings"
	"sync"
)

type GameGold struct {
	SteamId             int
	Name                string
	CopiesSold          int
	Unreleased          bool
	EarlyAccess         bool
	FirstReleaseDate    int
	ReleaseDate         int
	EarlyAccessExitDate int
	EAReleaseDate       int
	Developers          string
	Publishers          string
	PublisherClass      string
	ReviewScore         int
	Genres              string
	Categories          string
	Type                string
	IsFree              bool
	PriceCurrency       string
	PriceInitial        int
	PriceFinal          int
	PriceDiscount       int
	PlatformsWindows    bool
	PlatformsMac        bool
	PlatformsLinux      bool
	// ReleaseDateComingSoon bool
	// ReleaseDateDate       string
}
type PriceLogSilver struct {
	SteamId          int
	Timestamp        string
	PriceAmount      float32
	PriceAmountInt   int
	PriceCurrency    string
	RegularAmount    float32
	RegularAmountInt int
	RegularCurrency  string
	Cut              int
}

func transform(matrix GameMatrix) ([]GameGold, []PriceLogSilver) {
	var (
		games     []GameGold
		priceLogs []PriceLogSilver
	)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if !ctx.full {
			return
		}
		games = prepareGame(matrix)
	}()
	go func() {
		defer wg.Done()
		priceLogs = preparePriceLogs(matrix)
	}()
	wg.Wait()

	return games, priceLogs
}

func prepareGame(matrix GameMatrix) []GameGold {
	var games []GameGold
	for _, dim := range matrix {
		genres := make([]string, len(dim.Details.Genres))
		for i, genre := range dim.Details.Genres {
			genres[i] = genre.Name
		}
		genresStr := strings.Join(genres, ",")

		categories := make([]string, len(dim.Details.Categories))
		for i, categorie := range dim.Details.Categories {
			categories[i] = categorie.Name
		}
		categorieStr := strings.Join(categories, ",")

		game := GameGold{
			SteamId:             dim.Base.SteamId,
			Name:                dim.Details.Name,
			Unreleased:          dim.Base.Unreleased,
			EarlyAccess:         dim.Base.EarlyAccess,
			FirstReleaseDate:    dim.Base.FirstReleaseDate,
			ReleaseDate:         dim.Base.ReleaseDate,
			EarlyAccessExitDate: dim.Base.EarlyAccessExitDate,
			EAReleaseDate:       dim.Base.EAReleaseDate,
			Developers:          strings.Join(dim.Details.Developers, ","),
			Publishers:          strings.Join(dim.Details.Publishers, ","),
			PublisherClass:      dim.Base.PublisherClass,
			Genres:              genresStr,
			Categories:          categorieStr,
			Type:                dim.Details.Type,
			IsFree:              dim.Details.IsFree,
			PriceCurrency:       dim.Details.Price.Currency,
			PriceInitial:        dim.Details.Price.Initial,
			PriceFinal:          dim.Details.Price.Final,
			PriceDiscount:       dim.Details.Price.Discount,
			PlatformsWindows:    dim.Details.Platforms.Windows,
			PlatformsMac:        dim.Details.Platforms.Mac,
			PlatformsLinux:      dim.Details.Platforms.Linux,
		}
		games = append(games, game)
	}
	return games
}

func preparePriceLogs(matrix GameMatrix) []PriceLogSilver {
	var priceLogs []PriceLogSilver
	for i, dim := range matrix {
		for _, log := range dim.Prices {
			priceLog := PriceLogSilver{
				SteamId:          i,
				Timestamp:        log.Timestamp,
				PriceAmount:      log.Deal.Price.Amount,
				PriceAmountInt:   log.Deal.Price.AmountInt,
				PriceCurrency:    log.Deal.Price.Currency,
				RegularAmount:    log.Deal.Regular.Amount,
				RegularAmountInt: log.Deal.Regular.AmountInt,
				RegularCurrency:  log.Deal.Regular.Currency,
				Cut:              log.Deal.Cut,
			}
			priceLogs = append(priceLogs, priceLog)
		}
	}
	return priceLogs
}
