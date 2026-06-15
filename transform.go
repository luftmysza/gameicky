package main

import (
	"strings"
)

type GameSilver struct {
	SteamId             int
	Id                  int
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
	Type                string
	IsFree              bool
	PriceCurrency       string
	PriceInitial        int
	PriceFinal          int
	PriceDiscount       int
	PlatformsWindows    bool
	PlatformsMac        bool
	PlatformsLinux      bool
	Categories          string
	// ReleaseDateComingSoon bool
	// ReleaseDateDate       string
}
type PriceLogSilver struct {
}

func transform(matrix GameMatrix) ([]GameSilver, []PriceLogSilver) {
	games := prepareGame(matrix)

	priceLogs := preparePriceLogs(matrix)

	return games, priceLogs
}

func prepareGame(matrix GameMatrix) []GameSilver {
	var games []GameSilver

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

		game := GameSilver{
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
			Type:                dim.Details.Type,
			IsFree:              dim.Details.IsFree,
			PriceCurrency:       dim.Details.Price.Currency,
			PriceInitial:        dim.Details.Price.Initial,
			PriceFinal:          dim.Details.Price.Final,
			PriceDiscount:       dim.Details.Price.Discount,
			PlatformsWindows:    dim.Details.Platforms.Windows,
			PlatformsMac:        dim.Details.Platforms.Mac,
			PlatformsLinux:      dim.Details.Platforms.Linux,
			Categories:          categorieStr,
		}

		games = append(games, game)
	}

	return games
}

func preparePriceLogs(matrix GameMatrix) []PriceLogSilver {
	var priceLogs []PriceLogSilver

	return priceLogs
}
