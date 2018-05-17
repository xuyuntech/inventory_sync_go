package youzan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cabernety/gopkg/httplib"
	"golang.org/x/sync/errgroup"
)

var AccessToken = "0a678b1a989a3fdcb04fdb5cb5bdb0c3"

// Item 有赞商品信息
type Item struct {
	ItemID int64  `json:"item_id"`
	Title  string `json:"title"`
	ItemNO string `json:"item_no"`
}

type itemResponse struct {
	Response struct {
		Count int     `json:"count"`
		Items []*Item `json:"items"`
	} `json:"response"`
}

func queryItems(pageNo int, pageSize int, api string) ([]*Item, error) {
	response := &itemResponse{}
	res, err := httplib.Get(fmt.Sprintf("https://open.youzan.com/api/oauthentry/%s/3.0.0/get", api)).
		Param("access_token", AccessToken).
		Param("page_no", fmt.Sprintf("%d", pageNo)).
		Param("page_size", fmt.Sprintf("%d", pageSize)).
		SetTimeout(time.Second*10, time.Second*10).Response()
	if err != nil {
		return nil, err
	}
	b, _ := ioutil.ReadAll(res.Body)
	if err := json.Unmarshal(b, response); err != nil {
		return nil, err
	}
	// if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
	// 	return nil, err
	// }
	return response.Response.Items, nil
}

// QueryItems 获取出售中和仓库中的所有商品
func QueryItems() ([]*Item, error) {
	results := make([]*Item, 0)
	var g errgroup.Group
	g.Go(func() error {
		pageNo := 1
		pageSize := 100
		for {
			items, err := queryItems(pageNo, pageSize, "youzan.items.onsale")
			if err != nil {
				return err
			}
			results = append(results, items...)
			if len(items) < pageSize {
				return nil
			}
			pageNo++
		}
	})

	g.Go(func() error {
		pageNo := 1
		pageSize := 100
		for {
			items, err := queryItems(pageNo, pageSize, "youzan.items.inventory")
			if err != nil {
				return err
			}
			results = append(results, items...)
			if len(items) < pageSize {
				return nil
			}
			pageNo++
		}
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
