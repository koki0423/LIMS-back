package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// JANClient は外部APIとの通信を担当する
type JANClient struct {
	yahooAppID string
	httpClient *http.Client
}

func NewJANClient(appID string) *JANClient {
	return &JANClient{
		yahooAppID: appID,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// 内部用：Yahoo APIのレスポンス構造体
type yahooSearchResponse struct {
	Hits []struct {
		Name  string `json:"name"`
		Brand struct {
			Name string `json:"name"`
		} `json:"brand"`
	} `json:"hits"`
}

// 内部用：OpenBDのレスポンス構造体
type openBDResponse []struct {
	Summary struct {
		Title     string `json:"title"`
		Publisher string `json:"publisher"`
		Author    string `json:"author"`
	} `json:"summary"`
}

// FetchJANInfo はJAN/ISBNに応じた情報を取得する
func (c *JANClient) FetchJANInfo(ctx context.Context, janCode string) (JANLookupResponse, error) {
	isISBN := (strings.HasPrefix(janCode, "978") || strings.HasPrefix(janCode, "979")) && len(janCode) == 13

	if isISBN {
		res, err := c.lookupOpenBD(ctx, janCode)
		if err == nil {
			return res, nil
		}
	}
	return c.lookupYahoo(ctx, janCode)
}

func (c *JANClient) lookupOpenBD(ctx context.Context, isbn string) (JANLookupResponse, error) {
	url := "https://api.openbd.jp/v1/get?isbn=" + isbn
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return JANLookupResponse{}, err
	}
	defer resp.Body.Close()

	var obdRes openBDResponse
	if err := json.NewDecoder(resp.Body).Decode(&obdRes); err != nil {
		return JANLookupResponse{}, err
	}

	if len(obdRes) == 0 || obdRes[0].Summary.Title == "" {
		return JANLookupResponse{}, ErrNotFound("not found in openbd")
	}

	return JANLookupResponse{
		Name:         obdRes[0].Summary.Title,
		Manufacturer: obdRes[0].Summary.Publisher,
	}, nil
}

func (c *JANClient) lookupYahoo(ctx context.Context, janCode string) (JANLookupResponse, error) {
	if c.yahooAppID == "" {
		return JANLookupResponse{}, ErrInternal("Yahoo API AppID is not configured")
	}

	url := fmt.Sprintf("https://shopping.yahooapis.jp/ShoppingWebService/V3/itemSearch?appid=%s&jan_code=%s&results=1", c.yahooAppID, janCode)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return JANLookupResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return JANLookupResponse{}, ErrInternal(fmt.Sprintf("Yahoo API returned status: %d", resp.StatusCode))
	}

	var yRes yahooSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&yRes); err != nil {
		return JANLookupResponse{}, err
	}

	if len(yRes.Hits) == 0 {
		return JANLookupResponse{}, ErrNotFound("item not found by jan_code")
	}

	hit := yRes.Hits[0]
	name := cleanItemName(hit.Name)
	manufacturer := hit.Brand.Name

	if manufacturer == "" {
		parts := strings.Split(name, " ")
		if len(parts) > 0 {
			manufacturer = parts[0]
		}
	}

	return JANLookupResponse{
		Name:         name,
		Manufacturer: manufacturer,
	}, nil
}

func cleanItemName(name string) string {
	replacer := strings.NewReplacer(
		"【翌日発送】", "", "翌日発送・", "", "送料無料", "", "【新品】", "", "】", " ", "【", "",
	)
	return strings.TrimSpace(strings.ReplaceAll(replacer.Replace(name), "　", " "))
}