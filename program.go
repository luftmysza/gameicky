package main

import (
	// "database/sql"
	"bufio"
	"fmt"
	"strings"

	// "maps"
	"net/http"
	// "slices"
	"os"
	"strconv"
	"sync"
	"time"
	// _ "modernc.org/sqlite"
)

var (
	apiKeyITAD string
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Provide an ITAD API key:")
	input, err := reader.ReadString('\n')
	if err != nil {
		panicf("%s", err)
	}
	apiKeyITAD = strings.TrimSpace(input)

	matrix := extract()

	transform(matrix)

	load()
}

func extract() GameMatrix {
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
	appids = appids[:200]

	var (
		resSteam map[int]SteamResp
		resITAD  ITADResp
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
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
			panicf("Inital load from ITAD failed, %s", err)
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

	return matrix
}

func load() {

}
