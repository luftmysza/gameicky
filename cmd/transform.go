package main

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type GameDB struct {
	SteamId        int
	Name           string
	CopiesSold     int
	Unreleased     bool
	EarlyAccess    bool
	Developers     string
	Publishers     string
	PublisherClass string
	Genres         string
	Categories     string
	Type           string
	IsFree         bool
}

type PriceDB struct {
	SteamId       int
	PriceCurrency string
	PriceInitial  float32
	PriceFinal    float32
	PriceDiscount int
}

type DatesDB struct {
	SteamId             int
	FirstReleaseDate    string
	ReleaseDate         string
	EarlyAccessExitDate string
	EAReleaseDate       string
}

type PlatformsDB struct {
	SteamId          int
	PlatformsWindows bool
	PlatformsMac     bool
	PlatformsLinux   bool
}

type PriceLogsDB struct {
	SteamId          int
	TimestampUTC     string
	Timestamp        string
	PriceAmount      float32
	PriceAmountInt   int
	PriceCurrency    string
	RegularAmount    float32
	RegularAmountInt int
	RegularCurrency  string
	Cut              int
}

type ReviewsDB struct {
	SteamId     int
	ReviewScore int
	Metascore   int
	UserScore   float32
}

func transform(
	matrix GameMatrix,
	reviewsRaw []GameReview,
) ([]GameDB, []PriceDB, []DatesDB, []PlatformsDB, []PriceLogsDB, []ReviewsDB) {
	var (
		games     []GameDB
		prices    []PriceDB
		dates     []DatesDB
		platforms []PlatformsDB
		priceLogs []PriceLogsDB
		reviews   []ReviewsDB
	)

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()

		if !ctx.full {
			return
		}

		games, prices, dates, platforms = prepareGame(matrix)
	}()

	go func() {
		defer wg.Done()

		priceLogs = preparePriceLogs(matrix)
	}()

	go func() {
		defer wg.Done()

		if !ctx.full {
			return
		}

		reviews = matchAndPrepareReviews(matrix, reviewsRaw)
	}()

	wg.Wait()
	return games, prices, dates, platforms, priceLogs, reviews
}

func prepareGame(matrix GameMatrix) ([]GameDB, []PriceDB, []DatesDB, []PlatformsDB) {
	var (
		games     []GameDB
		prices    []PriceDB
		dates     []DatesDB
		platforms []PlatformsDB
	)

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

		game := GameDB{
			SteamId:        dim.Base.SteamId,
			Name:           strings.TrimSpace(dim.Details.Name),
			CopiesSold:     dim.Base.CopiesSold,
			Unreleased:     dim.Base.Unreleased,
			EarlyAccess:    dim.Base.EarlyAccess,
			Developers:     strings.Join(dim.Details.Developers, ","),
			Publishers:     strings.Join(dim.Details.Publishers, ","),
			PublisherClass: strings.TrimSpace(dim.Base.PublisherClass),
			Genres:         genresStr,
			Categories:     categorieStr,
			Type:           strings.TrimSpace(dim.Details.Type),
			IsFree:         dim.Details.IsFree,
		}
		games = append(games, game)

		price := PriceDB{
			SteamId:       dim.Base.SteamId,
			PriceCurrency: strings.TrimSpace(dim.Details.Price.Currency),
			PriceInitial:  float32(dim.Details.Price.Initial) / 100,
			PriceFinal:    float32(dim.Details.Price.Final) / 100,
			PriceDiscount: dim.Details.Price.Discount,
		}
		prices = append(prices, price)

		date := DatesDB{
			SteamId:             dim.Base.SteamId,
			FirstReleaseDate:    time.UnixMilli(dim.Base.FirstReleaseDate).UTC().Format(time.DateOnly),
			ReleaseDate:         time.UnixMilli(dim.Base.ReleaseDate).UTC().Format(time.DateOnly),
			EarlyAccessExitDate: time.UnixMilli(dim.Base.EarlyAccessExitDate).UTC().Format(time.DateOnly),
			EAReleaseDate:       time.UnixMilli(dim.Base.EAReleaseDate).UTC().Format(time.DateOnly),
		}
		dates = append(dates, date)

		platform := PlatformsDB{
			SteamId:          dim.Base.SteamId,
			PlatformsWindows: dim.Details.Platforms.Windows,
			PlatformsMac:     dim.Details.Platforms.Mac,
			PlatformsLinux:   dim.Details.Platforms.Linux,
		}
		platforms = append(platforms, platform)
	}
	return games, prices, dates, platforms
}

func preparePriceLogs(matrix GameMatrix) []PriceLogsDB {
	var priceLogs []PriceLogsDB
	for i, dim := range matrix {
		for _, log := range dim.Prices {
			priceLog := PriceLogsDB{
				SteamId:          i,
				TimestampUTC:     log.Timestamp,
				Timestamp:        strings.Replace(log.Timestamp[:19], "T", " ", 1),
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
func matchAndPrepareReviews(
	matrix GameMatrix,
	reviews []GameReview,
) []ReviewsDB {
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))

		var result strings.Builder
		lastWasSpace := false

		for _, char := range value {
			switch {
			case unicode.IsLetter(char), unicode.IsDigit(char):
				result.WriteRune(char)
				lastWasSpace = false

			case char == '&':
				if result.Len() > 0 && !lastWasSpace {
					result.WriteByte(' ')
				}

				result.WriteString("and")
				result.WriteByte(' ')
				lastWasSpace = true

			default:
				if result.Len() > 0 && !lastWasSpace {
					result.WriteByte(' ')
					lastWasSpace = true
				}
			}
		}

		normalized := strings.Join(
			strings.Fields(result.String()),
			" ",
		)

		removablePhrases := []string{
			"game of the year edition",
			"goty edition",
			"definitive edition",
			"complete edition",
			"ultimate edition",
			"deluxe edition",
			"enhanced edition",
			"remastered edition",
		}

		for _, phrase := range removablePhrases {
			normalized = strings.ReplaceAll(
				normalized,
				phrase,
				"",
			)
		}

		return strings.Join(
			strings.Fields(normalized),
			" ",
		)
	}

	levenshtein := func(left, right string) int {
		leftRunes := []rune(left)
		rightRunes := []rune(right)

		if len(leftRunes) == 0 {
			return len(rightRunes)
		}

		if len(rightRunes) == 0 {
			return len(leftRunes)
		}

		previous := make([]int, len(rightRunes)+1)
		current := make([]int, len(rightRunes)+1)

		for i := range previous {
			previous[i] = i
		}

		for i, leftChar := range leftRunes {
			current[0] = i + 1

			for j, rightChar := range rightRunes {
				cost := 0
				if leftChar != rightChar {
					cost = 1
				}

				deletion := previous[j+1] + 1
				insertion := current[j] + 1
				substitution := previous[j] + cost

				current[j+1] = min(
					deletion,
					insertion,
					substitution,
				)
			}

			previous, current = current, previous
		}

		return previous[len(rightRunes)]
	}

	editSimilarity := func(left, right string) float64 {
		if left == right {
			return 1
		}

		maxLength := max(
			len([]rune(left)),
			len([]rune(right)),
		)

		if maxLength == 0 {
			return 1
		}

		distance := levenshtein(left, right)

		return 1 - float64(distance)/float64(maxLength)
	}

	tokenSimilarity := func(left, right string) float64 {
		leftSet := make(map[string]struct{})
		rightSet := make(map[string]struct{})

		for _, token := range strings.Fields(left) {
			leftSet[token] = struct{}{}
		}

		for _, token := range strings.Fields(right) {
			rightSet[token] = struct{}{}
		}

		if len(leftSet) == 0 || len(rightSet) == 0 {
			return 0
		}

		intersection := 0

		for token := range leftSet {
			if _, exists := rightSet[token]; exists {
				intersection++
			}
		}

		union := len(leftSet) + len(rightSet) - intersection

		return float64(intersection) / float64(union)
	}

	titleSimilarity := func(left, right string) float64 {
		left = normalize(left)
		right = normalize(right)

		if left == "" || right == "" {
			return 0
		}

		if left == right {
			return 1
		}

		editScore := editSimilarity(left, right)
		tokenScore := tokenSimilarity(left, right)

		return math.Max(editScore, tokenScore)
	}

	parseMetascore := func(value string) int {
		value = strings.TrimSpace(value)

		if value == "" || strings.EqualFold(value, "tbd") {
			return 0
		}

		score, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}

		return score
	}

	parseUserScore := func(value string) float32 {
		value = strings.TrimSpace(value)

		if value == "" || strings.EqualFold(value, "tbd") {
			return 0
		}

		score, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return 0
		}

		return float32(score)
	}

	developerMatches := func(
		gameDevelopers []string,
		reviewDeveloper string,
	) bool {
		reviewDeveloper = normalize(reviewDeveloper)

		if reviewDeveloper == "" {
			return false
		}

		for _, developer := range gameDevelopers {
			gameDeveloper := normalize(developer)

			if gameDeveloper == reviewDeveloper {
				return true
			}

			if titleSimilarity(
				gameDeveloper,
				reviewDeveloper,
			) >= 0.90 {
				return true
			}
		}

		return false
	}

	pcReviews := make([]GameReview, 0, len(reviews))

	for _, review := range reviews {
		if strings.EqualFold(
			strings.TrimSpace(review.Platform),
			"PC",
		) {
			pcReviews = append(pcReviews, review)
		}
	}

	prepared := make([]ReviewsDB, 0, len(matrix))

	const (
		minimumScore  = 0.82
		minimumMargin = 0.08
	)

	for appid, dim := range matrix {
		result := ReviewsDB{
			SteamId:     appid,
			ReviewScore: dim.Base.ReviewScore,
		}

		gameName := strings.TrimSpace(dim.Details.Name)

		if gameName == "" {
			gameName = strings.TrimSpace(dim.Base.Name)
		}

		if gameName == "" {
			prepared = append(prepared, result)
			continue
		}

		normalizedGameName := normalize(gameName)

		bestIndex := -1
		bestScore := 0.0
		secondBestScore := 0.0

		for reviewIndex, review := range pcReviews {
			normalizedReviewName := normalize(review.Name)

			if normalizedReviewName == "" {
				continue
			}

			score := titleSimilarity(
				normalizedGameName,
				normalizedReviewName,
			)

			if score >= 0.65 &&
				developerMatches(
					dim.Base.Developers,
					review.Developer,
				) {
				score += 0.05

				if score > 1 {
					score = 1
				}
			}

			if score > bestScore {
				secondBestScore = bestScore
				bestScore = score
				bestIndex = reviewIndex
			} else if score > secondBestScore {
				secondBestScore = score
			}
		}

		isExact := bestIndex >= 0 &&
			normalize(pcReviews[bestIndex].Name) ==
				normalizedGameName

		isConfidentFuzzyMatch := bestIndex >= 0 &&
			bestScore >= minimumScore &&
			bestScore-secondBestScore >= minimumMargin

		if isExact || isConfidentFuzzyMatch {
			matchedReview := pcReviews[bestIndex]

			result.Metascore = parseMetascore(
				matchedReview.Metascore,
			)

			result.UserScore = parseUserScore(
				matchedReview.UserScore,
			)
		}

		prepared = append(prepared, result)
	}

	return prepared
}
