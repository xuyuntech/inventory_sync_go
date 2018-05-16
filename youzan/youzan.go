package youzan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/cabernety/gopkg/httplib"
	"golang.org/x/sync/errgroup"
)

var accessToken = "fec1c42e77333ad3a7d96726c1d268b6"

// Item 有赞商品信息
type Item struct {
	ItemID string
	Title  string
	ItemNO string
}

func queryItems(pageNo int, pageSize int, api string) ([]*Item, error) {
	var response struct {
		response struct {
			count int
			items []*Item
		}
	}
	res, err := httplib.Get(fmt.Sprintf("https://open.youzan.com/api/oauthentry/%s/3.0.0/get", api)).
		Param("access_token", accessToken).
		Param("page_no", fmt.Sprintf("%d", pageNo)).
		Param("page_size", fmt.Sprintf("%d", pageSize)).
		SetTimeout(time.Second*10, time.Second*10).Response()
	if err != nil {
		return nil, err
	}
	b, _ := ioutil.ReadAll(res.Body)
	if err := json.Unmarshal(b, &response); err != nil {
		return nil, err
	}
	logrus.Debugf("b-> %s", string(b))
	// if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
	// 	return nil, err
	// }
	logrus.Debugf("response %+v", response)
	return response.response.items, nil
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
