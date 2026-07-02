package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GameMatrix map[int]Dim
type Dim struct {
	Base    GamalyticGame
	Details SteamGame
	Prices  []ITADLog
}

type SteamResp map[int]SteamWrap
type SteamWrap struct {
	Success bool      `json:"success"`
	Data    SteamGame `json:"data"`
}
type SteamGame struct {
	SteamAppid int      `json:"steam_appid"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	IsFree     bool     `json:"is_free"`
	Developers []string `json:"developers"`
	Publishers []string `json:"publishers"`
	Price      struct {
		Currency string `json:"currency"`
		Initial  int    `json:"initial"`
		Final    int    `json:"final"`
		Discount int    `json:"discount_percent"`
	} `json:"price_overview"`
	Platforms struct {
		Windows bool `json:"windows"`
		Mac     bool `json:"mac"`
		Linux   bool `json:"linux"`
	} `json:"platforms"`
	Genres []struct {
		Name string `json:"description"`
	} `json:"genres"`
	Categories []struct {
		Name string `json:"description"`
	} `json:"categories"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
}

type ITADResp map[int]ITADSnapshot
type ITADSnapshot struct {
	guid      string
	priceLogs []ITADLog
}
type ITADLog struct {
	Timestamp string `json:"timestamp"`
	Shop      struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"shop"`
	Deal struct {
		Price struct {
			Amount    float32 `json:"amount"`
			AmountInt int     `json:"amountInt"`
			Currency  string  `json:"currency"`
		} `json:"price"`
		Regular struct {
			Amount    float32 `json:"amount"`
			AmountInt int     `json:"amountInt"`
			Currency  string  `json:"currency"`
		} `json:"regular"`
		Cut int `json:"cut"`
	} `json:"deal"`
}

type GamalyticProc map[int]GamalyticGame
type GamalyticResp struct {
	Pages  int             `json:"pages"`
	Total  int             `json:"total"`
	Result []GamalyticGame `json:"result"`
}
type GamalyticGame struct {
	SteamId             int      `json:"steamId"`
	Id                  int      `json:"id"`
	Name                string   `json:"name"`
	CopiesSold          int      `json:"copiesSold"`
	Unreleased          bool     `json:"unreleased"`
	EarlyAccess         bool     `json:"earlyAccess"`
	FirstReleaseDate    int64    `json:"firstReleaseDate"`
	ReleaseDate         int64    `json:"releaseDate"`
	EarlyAccessExitDate int64    `json:"earlyAccessExitDate"`
	EAReleaseDate       int64    `json:"EAReleaseDate"`
	Price               float32  `json:"price"`
	Developers          []string `json:"developers"`
	Publishers          []string `json:"publishers"`
	PublisherClass      string   `json:"publisherClass"`
	ReviewScore         int      `json:"reviewScore"`
	Genres              []string `json:"genres"`
}

type GameReview struct {
	Name        string
	Platform    string
	ReleaseDate string
	Metascore   string
	UserScore   string
	Developer   string
	Publisher   string
	Genre       string
}

func extract() (GameMatrix, []GameReview) {
	zhttp := httpCustom{}
	zhttp.client = &http.Client{Timeout: 30 * time.Second}

	resGamalytic, err := fetchGamesList(&zhttp)
	if err != nil {
		panicf("Inital load from SteamSpy failed, %s", err)
	}

	var (
		appids   []int
		gamesMap GamalyticProc
	)
	gamesMap = make(GamalyticProc)
	for i := 0; i < len(resGamalytic.Result); i++ {
		game := resGamalytic.Result[i]
		appid := game.SteamId
		appids = append(appids, appid)
		gamesMap[appid] = game
	}
	// appids = appids[:x] // for testing purposes

	var (
		resSteam map[int]SteamResp
		resITAD  ITADResp
		reviews  []GameReview
		wg       sync.WaitGroup
	)
	wg.Add(3)

	go func() {
		defer wg.Done()

		if !ctx.full {
			return
		}

		resSteam, err = fetchGameDetails(&zhttp, appids)
		if err != nil {
			panicf("Inital load from Steam failed, %s", err)
		}
	}()
	go func() {
		defer wg.Done()

		params := make(map[int]string)
		for _, appid := range appids {
			params[appid] = fmt.Sprintf("app/%v", strconv.Itoa(appid))
		}

		resITAD, err = fetchGamesPrices(&zhttp, params)
		if err != nil {
			panicf("Load from ITAD failed, %s", err)
		}
	}()
	go func() {
		defer wg.Done()

		if !ctx.full {
			return
		}

		reviews, err = loadGamesReviews()

		if err != nil {
			panicf("Load from CSV failed, %s", err)
		}
	}()
	wg.Wait()

	matrix := make(GameMatrix)
	for _, appid := range appids {
		matrix[appid] = Dim{
			Base:    gamesMap[appid],
			Details: resSteam[appid][appid].Data,
			Prices:  resITAD[appid].priceLogs,
		}
	}

	return matrix, reviews
}

func fetchGameDetails(client *httpCustom, appids []int) (map[int]SteamResp, error) {
	const urlBase = "https://store.steampowered.com/api/appdetails"
	apiRespList := make(map[int]SteamResp)

	for _, appid := range appids {
		reqURL := fmt.Sprintf("%s?appids=%d&cc=us", urlBase, appid)
		res, err := client.get(reqURL)
		if err != nil {
			msg := serrorf("cannot fetch appid %d: %v", appid, err)
			log.Print(msg)
		}
		res = bytes.TrimSpace(res)
		res = bytes.TrimPrefix(res, []byte{0xEF, 0xBB, 0xBF})

		var apiResp SteamResp
		if err := json.Unmarshal(res, &apiResp); err != nil {
			msg := serrorf("cannot decode appid %d: %v", appid, err)
			log.Print(msg)
		}

		apiRespList[appid] = apiResp
		msg := sinfof("Collected game details for %d", appid)
		log.Print(msg)

		time.Sleep(time.Millisecond * 1500)
	}
	return apiRespList, nil
}

func fetchGamesPrices(client *httpCustom, params map[int]string) (ITADResp, error) {
	const urlHistoryBase = "https://api.isthereanydeal.com/games/history/v2"
	const urlLookupBase = "https://api.isthereanydeal.com/lookup/id/shop/61/v1"

	url := urlLookupBase
	appidsStr := slices.Collect(maps.Values(params))
	body, err := json.Marshal(appidsStr)
	if err != nil {
		msg := serrorf("cannot unmarshal the appids: %v", err)
		return nil, errors.New(msg)
	}
	res, err := client.post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		msg := serrorf("cannot marshal the appids: %v", err)
		return nil, errors.New(msg)
	}
	res = bytes.TrimSpace(res)
	res = bytes.TrimPrefix(res, []byte{0xEF, 0xBB, 0xBF})

	var mapGuids map[string]string
	if err := json.Unmarshal(res, &mapGuids); err != nil {
		msg := serrorf("cannot unmarshal the API response: %v", err)
		return nil, errors.New(msg)
	}

	since := time.Now().AddDate(-2, 0, 0).UTC().Format(time.RFC3339)
	apiResp := make(ITADResp)

	for appid, appidStr := range params {
		guid := mapGuids[appidStr]
		url = fmt.Sprintf("%s?&key=%s&id=%s&shops=61&country=US&since=%v", urlHistoryBase, apiKeyITAD, guid, since)
		res, err := client.get(url)
		if err != nil {
			msg := serrorf("cannot fetch game %d: %v", appid, err)
			log.Print(msg)
		}
		var priceLogs []ITADLog
		if err := json.Unmarshal(res, &priceLogs); err != nil {
			msg := serrorf("cannot unmarchal game %d: %v", appid, err)
			log.Print(msg)
		}

		apiResp[appid] = ITADSnapshot{guid: guid, priceLogs: priceLogs}
		msg := sinfof("Collected price logs for %d", appid)
		log.Print(msg)

		time.Sleep(time.Millisecond * 3000)
	}
	return apiResp, nil
}

func fetchGamesList(client *httpCustom) (GamalyticResp, error) {
	const urlBase = "https://api.gamalytic.com/steam-games/list"
	var apiRespFull GamalyticResp

	for page := range 1 {

		reqURL := fmt.Sprintf("%s?page=%d&limit=1000&sort_mode=desc&unreleased=false&release_status=released", urlBase, page)
		res, err := client.get(reqURL)
		if err != nil {
			msg := serrorf("cannot fetch page %d: %s", page, err)
			return apiRespFull, errors.New(msg)
		}

		var apiResp GamalyticResp
		if err := json.Unmarshal(res, &apiResp); err != nil {
			msg := serrorf("cannot decode page %d: %s", page, err)
			return apiRespFull, errors.New(msg)
		}
		if apiRespFull.Pages == 0 {
			apiRespFull.Pages = apiResp.Pages
			apiRespFull.Total = apiResp.Total
		}
		apiRespFull.Result = slices.Concat(apiRespFull.Result, apiResp.Result)
	}
	return apiRespFull, nil
}

func loadGamesReviews() ([]GameReview, error) {
	file, err := os.Open("data/game_reviews.csv")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	if _, err := reader.Read(); err != nil {
		return nil, err
	}

	var reviews []GameReview

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(record) < 8 {
			continue
		}

		if strings.TrimSpace(record[1]) != "PC" {
			continue
		}

		reviews = append(reviews, GameReview{
			Name:        strings.TrimSpace(record[0]),
			Platform:    strings.TrimSpace(record[1]),
			ReleaseDate: strings.TrimSpace(record[2]),
			Metascore:   strings.TrimSpace(record[3]),
			UserScore:   strings.TrimSpace(record[4]),
			Developer:   strings.TrimSpace(record[5]),
			Publisher:   strings.TrimSpace(record[6]),
			Genre:       strings.TrimSpace(record[7]),
		})
	}

	return reviews, nil
}
