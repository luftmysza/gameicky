package main

type Game struct {
	ID                int
	Name              string
	Type              string
	Developers        []string
	Publishers        []string
	Genres            []string
	Categories        []string
	Tags              map[string]int
	Windows           bool
	Mac               bool
	Linux             bool
	IsFree            bool
	CurrentPriceCents int
	Currency          string
	PositiveReviews   int
	NegativeReviews   int
	Owners            string
	CCU               int
}

type PriceWindow struct {
	ID                int
	Window            string
	LowPriceCents     int
	RegularPriceCents int
	Currency          string
	DiscountPercent   int
	ShopID            int
	ShopName          string
	LowAt             string
}
