package youzan

import (
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrOfflineNotFound = errors.New("Offline not found")
)

type Offline struct {
	OfflineID string `json:"id"`
	Name      string `json:"name"`
}

// Offlines 网点列表
type Offlines struct {
	Items []*Offline
}

// OfflineItem 网点商品
type GoodsDetail struct {
	NumIID  string      `json:"num_iid"`
	Title   string      `json:"title"`
	Skus    []*GoodsSku `json:"skus"`
	OuterID string      `json:"outer_id"`
}

type GoodsSku struct {
	OuterID            string  `json:"outer_id"`
	SkuID              int64   `json:"sku_id"`
	Quantity           float64 `json:"-"`
	QuantityStr        string  `json:"quantity"`
	Price              float64 `json:"-"`
	PriceStr           string  `json:"price"`
	PropertiesNameJSON string  `json:"properties_name_json"`
}

type GoodSkuProperty struct {
	Kid string `json:"kid"`
	Vid string `json:"vid"`
	K   string `json:"k"`
	V   string `json:"v"`
}

func (gs *GoodsSku) FormatProperties() ([]*GoodSkuProperty, error) {
	m := make([]*GoodSkuProperty, 0)
	if err := json.Unmarshal([]byte(gs.PropertiesNameJSON), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (o *Offlines) GetIDByName(name string) (string, error) {
	for _, offline := range o.Items {
		if strings.Trim(offline.Name, " ") == strings.Trim(name, " ") {
			return offline.OfflineID, nil
		}
	}
	return "", ErrOfflineNotFound
}

// Item 有赞商品信息
type Item struct {
	ItemID int64  `json:"item_id"`
	Title  string `json:"title"`
	ItemNO string `json:"item_no"`
}
