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

type itemResponse struct {
	Response struct {
		Count int     `json:"count"`
		Items []*Item `json:"items"`
	} `json:"response"`
}

type offlineResponse struct {
	Response struct {
		Count int        `json:"count"`
		List  []*Offline `json:"list"`
	} `json:"response"`
}

type offlineItemResponse struct {
	Response struct {
		Item *GoodsDetail `json:"item"`
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

func queryOfflines(pageNo int, pageSize int) ([]*Offline, error) {
	response := &offlineResponse{}
	res, err := httplib.Get(fmt.Sprintf("https://open.youzan.com/api/oauthentry/youzan.multistore.offline/3.0.0/search")).
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
	return response.Response.List, nil
}

func QueryOfflines() (*Offlines, error) {
	offlines := &Offlines{}
	results := make([]*Offline, 0)
	pageNo := 1
	pageSize := 100
	for {
		offlines, err := queryOfflines(pageNo, pageSize)
		if err != nil {
			return nil, err
		}
		results = append(results, offlines...)
		if len(offlines) < pageSize {
			break
		}
		pageNo++
	}
	offlines.Items = results
	return offlines, nil
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
