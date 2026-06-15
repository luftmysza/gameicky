package main

import (
	// "database/sql"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

type GameMatrix map[int]Dim
type Dim struct {
	Base    GamalyticGame
	Details SteamGame
	Prices  []ITADLog
}

type SSpyResp map[int]SSpyGame
type SSpyGame struct {
	Appid          int    `json:"appid"`
	Name           string `json:"name"`
	Developer      string `json:"developer"`
	Publisher      string `json:"publisher"`
	ScoreRank      string `json:"score_rank"`
	Positive       int    `json:"positive"`
	Negative       int    `json:"negative"`
	Userscore      int    `json:"userscore"`
	Owners         string `json:"owners"`
	AverageForever int    `json:"average_forever"`
	Average2Weeks  int    `json:"averege_2weeks"`
	MedianForever  int    `json:"median_average"`
	Median2Weeks   int    `json:"median_2weeks"`
	Price          string `json:"price"`
	Initialprice   string `json:"initialprice"`
	Discount       string `json:"discount"`
	CCU            int    `json:"ccu"`
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
	FirstReleaseDate    int      `json:"firstReleaseDate"`
	ReleaseDate         int      `json:"releaseDate"`
	EarlyAccessExitDate int      `json:"earlyAccessExitDate"`
	EAReleaseDate       int      `json:"EAReleaseDate"`
	Price               int      `json:"price"`
	Developers          []string `json:"developers"`
	Publishers          []string `json:"publishers"`
	PublisherClass      string   `json:"publisherClass"`
	ReviewScore         int      `json:"reviewScore"`
	Genres              []string `json:"genres"`
}

func obsolete_fetchGamesList(client *httpCustom) (SSpyResp, error) {
	const urlBase = "https://steamspy.com/api.php"
	apiRespList := make(SSpyResp)

	for page := range 1 {
		reqURL := fmt.Sprintf("%s?request=all&page=%d", urlBase, page)
		res, err := client.get(reqURL)
		if err != nil {
			msg := serrorf("cannot fetch page %d: %s", page, err)
			return nil, errors.New(msg)
		}

		var apiResp SSpyResp
		if err := json.Unmarshal(res, &apiResp); err != nil {
			msg := serrorf("cannot decode page %d: %s", page, err)
			return nil, errors.New(msg)
		}
		maps.Copy(apiRespList, apiResp)
	}
	return apiRespList, nil
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
			// continue
		}

		var apiResp SteamResp
		if err := json.Unmarshal(res, &apiResp); err != nil {
			msg := serrorf("cannot decode appid %d: %v", appid, err)
			log.Print(msg)
			// continue
		}

		apiRespList[appid] = apiResp
		msg := sinfof("Collected game details for %d:", appid)
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
		msg := sinfof("Collected price logs for  %d", appid)
		log.Print(msg)

		time.Sleep(time.Millisecond * 300)
	}
	return apiResp, nil
}

func fetchGamesList(client *httpCustom) (GamalyticResp, error) {
	const urlBase = "https://api.gamalytic.com/steam-games/list"
	apiRespFull := GamalyticResp{}

	for page := range 1 {

		reqURL := fmt.Sprintf("%s?page=%d&limit=1000&sort_mode=desc&unreleased=false&release_status=released", urlBase, page)
		res, err := client.get(reqURL)
		if err != nil {
			msg := serrorf("cannot fetch page %d: %s", page, err)
			return apiRespFull, errors.New(msg)
		}

		var apiResp []GamalyticGame
		if err := json.Unmarshal(res, &apiResp); err != nil {
			msg := serrorf("cannot decode page %d: %s", page, err)
			return apiRespFull, errors.New(msg)
		}
		apiRespFull.Result = slices.Concat(apiRespFull.Result, apiResp)
	}
	return apiRespFull, nil
}
