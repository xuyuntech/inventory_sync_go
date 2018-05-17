package youzan

import "errors"

type Offline struct {
	OfflineID int64  `json:"id"`
	Name      string `json:"name"`
}

// Offlines 网点列表
type Offlines struct {
	Items []*Offline
}

// OfflineItem 网点商品
type GoodsDetail struct {
	NumIID  int64       `json:"num_iid"`
	Title   string      `json:"title"`
	Skus    []*GoodsSku `json:"skus"`
	OuterID string      `json:"outer_id"`
}

type GoodsSku struct {
	OuterID            string  `json:"outer_id"`
	SkuID              int64   `json:"sku_id"`
	Quantity           float64 `json:"quantity"`
	PropertiesNameJSON string  `json:"properties_name_json"`
	Price              float64 `json:"price"`
}

var (
	ErrOfflineNotFound = errors.New("Offline not found")
)

func (o *Offlines) GetIDByName(name string) (int64, error) {
	for _, offline := range o.Items {
		if offline.Name == name {
			return offline.OfflineID, nil
		}
	}
	return 0, ErrOfflineNotFound
}

// Item 有赞商品信息
type Item struct {
	ItemID int64  `json:"item_id"`
	Title  string `json:"title"`
	ItemNO string `json:"item_no"`
}
