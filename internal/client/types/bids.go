package types

type BidsSliceWrapper struct {
	BidWrappers []BidWrapper `json:"bids"`
}

type BidWrapper struct {
	Bid Bid `json:"bid"`
}

type Bids []Bid

type Bid struct {
	Id    BidId    `json:"bid_id"`
	Price BidPrice `json:"price"`
	State string   `json:"state,omitempty"`
	CreatedAt int64 `json:"created_at,omitempty"`
}

type BidId struct {
	Owner    string `json:"owner"`
	Dseq     string `json:"dseq"`
	Gseq     string `json:"gseq"`
	Oseq     string `json:"oseq"`
	Provider string `json:"provider"`
}

type BidPrice struct {
	Denom  string  `json:"denom"`
	Amount float32 `json:"amount,string"`
}

func (b Bids) GetProviderAddresses() []string {
	addresses := make([]string, 0, len(b))

	for _, bid := range b {
		addresses = append(addresses, bid.Id.Provider)
	}

	return addresses
}

func (b Bids) FindByProvider(provider string) Bid {
	for _, bid := range b {
		if bid.Id.Provider == provider {
			return bid
		}
	}

	return Bid{}
}

// FindAllByProviders finds all the Bid structures that have any of the given providers.
// It returns a slice of all the Bid structures where the providers were found.
func (b Bids) FindAllByProviders(providers []string) Bids {
	bids := make(Bids, 0)

	for _, provider := range providers {
		if bid := b.FindByProvider(provider); bid != (Bid{}) {
			bids = append(bids, bid)
		}
	}

	return bids
}

// GetLowestPriceBid returns the bid with the lowest price from a list of bids
func (b Bids) GetLowestPriceBid() *Bid {
	if len(b) == 0 {
		return nil
	}

	var lowestBid *Bid
	var lowestPrice float32 = -1

	for i := range b {
		bid := &b[i]
		if lowestPrice == -1 || bid.Price.Amount < lowestPrice {
			lowestPrice = bid.Price.Amount
			lowestBid = bid
		}
	}

	return lowestBid
}

// FilterByMaxPrice filters bids that are at or below the maximum price (in the same denomination)
func (b Bids) FilterByMaxPrice(maxPrice float32, denom string) Bids {
	filtered := Bids{}
	
	for _, bid := range b {
		// Only include bids with matching denomination and price <= maxPrice
		if bid.Price.Denom == denom && bid.Price.Amount <= maxPrice {
			filtered = append(filtered, bid)
		}
	}

	return filtered
}
