package assets

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}

	// masters
	r.POST("/assets/masters", h.CreateAssetMaster)
	r.GET("/assets/masters", h.ListAssetMasters)
	r.GET("/assets/masters/:management_number", h.GetAssetMaster)
	r.PUT("/assets/masters/:management_number", h.UpdateAssetMaster)

	// assets
	r.POST("/assets", h.CreateAsset)
	r.GET("/assets", h.ListAssets)
	r.GET("/assets/:asset_id", h.GetAsset)
	r.PUT("/assets/:asset_id", h.UpdateAsset)

	// assest-set
	r.POST("/assets/pair", h.RegisterAsset) // 便宜上．/assets/pair という名前にしているが，旧実装を廃止したら POST /assets に変更．
	r.GET("/assets/pair/:management_number", h.GetAssetSet)
	r.POST("/assets/import", h.HandleImportAssets) //curl -X POST "http://localhost:8443/api/v2/assets/import?mode=commit" -F "file=@./asset.csv"

	// search
	r.GET("/assets/search", h.SearchAssets)

	// JANコード検索
	r.GET("/assets/lookup/:jan_code", h.LookupJAN)
}

// ===== masters =====

// @Summary      Create a new asset master
// @Description  Creates a new master record for an asset type. The management_number is generated automatically.
// @Tags         assets-masters
// @Accept       json
// @Produce      json
// @Param        assetMaster body CreateAssetMasterRequest true "Asset Master to create"
// @Success      201 {object} AssetMasterResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      409 {object} ErrorResponse "Conflict, e.g., duplicate management_number on generation"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/masters [post]
func (h *Handler) CreateAssetMaster(c *gin.Context) {
	var req CreateAssetMasterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAssetMaster(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/assets/masters/"+res.ManagementNumber)
	c.JSON(http.StatusCreated, res)
}

// @Summary      Get an asset master
// @Description  Get details of an asset master by its management number.
// @Tags         assets-masters
// @Produce      json
// @Param        management_number path string true "Management Number"
// @Success      200 {object} AssetMasterResponse
// @Failure      404 {object} ErrorResponse "Asset master not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/masters/{management_number} [get]
func (h *Handler) GetAssetMaster(c *gin.Context) {
	mng := c.Param("management_number")
	res, err := h.svc.GetAssetMaster(c.Request.Context(), mng)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// @Summary      List asset masters
// @Description  Get a paginated list of asset masters.
// @Tags         assets-masters
// @Produce      json
// @Param        genre   query int false "Filter by genre ID"
// @Param        limit   query int false "Number of items to return" default(50)
// @Param        offset  query int false "Offset for pagination"
// @Param        order   query string false "Sort order ('asc' or 'desc')" Enums(asc, desc) default(desc)
// @Success      200 {object} ListAssetMastersResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/masters [get]
func (h *Handler) ListAssetMasters(c *gin.Context) {
	var q AssetSearchQuery
	if v := c.Query("genre"); v != "" {
		id, err := strconv.Atoi(v)
		if err == nil {
			u := uint(id)
			q.GenreID = &u
		}
	}
	p := Page{
		Limit:  atoiDef(c.Query("limit"), 50),
		Offset: atoiDef(c.Query("offset"), 0),
		Order:  strings.ToLower(c.DefaultQuery("order", "desc")),
	}
	items, total, err := h.svc.ListAssetMasters(c.Request.Context(), p, q)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "next_offset": nextOffset(total, p)})
}

// @Summary      Update an asset master
// @Description  Update details of an existing asset master.
// @Tags         assets-masters
// @Accept       json
// @Produce      json
// @Param        management_number path string true "Management Number"
// @Param        assetMaster body UpdateAssetMasterRequest true "Fields to update"
// @Success      200 {object} AssetMasterResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Asset master not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/masters/{management_number} [put]
func (h *Handler) UpdateAssetMaster(c *gin.Context) {
	mng := c.Param("management_number")
	var req UpdateAssetMasterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.UpdateAssetMaster(c.Request.Context(), mng, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== assets =====

// @Summary      Create a new asset instance
// @Description  Creates a new instance of an asset, linked to an asset master.
// @Tags         assets
// @Accept       json
// @Produce      json
// @Param        asset body CreateAssetRequest true "Asset to create"
// @Success      201 {object} AssetResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets [post]
func (h *Handler) CreateAsset(c *gin.Context) {
	var req CreateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateAsset: bind error: %v", err)
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAsset(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/assets/"+strconv.FormatUint(res.AssetID, 10))
	c.JSON(http.StatusCreated, res)
}

// @Summary      Get an asset instance
// @Description  Get details of an asset instance by its ID.
// @Tags         assets
// @Produce      json
// @Param        asset_id path int true "Asset ID"
// @Success      200 {object} AssetResponse
// @Failure      400 {object} ErrorResponse "Invalid asset ID"
// @Failure      404 {object} ErrorResponse "Asset not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/{asset_id} [get]
func (h *Handler) GetAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("asset_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "asset_id must be a number"))
		return
	}
	res, err := h.svc.GetAsset(c.Request.Context(), id)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// @Summary      List asset instances
// @Description  Get a paginated list of asset instances with filtering.
// @Tags         assets
// @Produce      json
// @Param        management_number query string false "Filter by management number"
// @Param        asmi              query int false "Filter by asset master ID"
// @Param        status_id         query int false "Filter by status ID"
// @Param        owner             query string false "Filter by owner"
// @Param        location          query string false "Filter by location"
// @Param        purchased_from    query string false "Filter by purchased date (start, YYYY-MM-DD)" Format(date)
// @Param        purchased_to      query string false "Filter by purchased date (end, YYYY-MM-DD)" Format(date)
// @Param        limit             query int false "Number of items to return" default(50)
// @Param        offset            query int false "Offset for pagination"
// @Param        order             query string false "Sort order ('asc' or 'desc')" Enums(asc, desc) default(desc)
// @Success      200 {object} ListAssetsResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets [get]
func (h *Handler) ListAssets(c *gin.Context) {
	var q AssetSearchQuery
	if v := c.Query("management_number"); v != "" {
		q.ManagementNumber = &v
	}
	if v := c.Query("asmi"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q.AssetMasterID = &n
		}
	}
	if v := c.Query("status_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			u := uint(n)
			q.StatusID = &u
		}
	}
	if v := c.Query("owner"); v != "" {
		q.Owner = &v
	}
	if v := c.Query("location"); v != "" {
		q.Location = &v
	}
	if v := c.Query("purchased_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.PurchasedFrom = &t
		}
	}
	if v := c.Query("purchased_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.PurchasedTo = &t
		}
	}

	p := Page{
		Limit:  atoiDef(c.Query("limit"), 50),
		Offset: atoiDef(c.Query("offset"), 0),
		Order:  strings.ToLower(c.DefaultQuery("order", "desc")),
	}
	items, total, err := h.svc.ListAssets(c.Request.Context(), q, p)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "next_offset": nextOffset(total, p)})
}

// @Summary      Update an asset instance
// @Description  Update details of an existing asset instance.
// @Tags         assets
// @Accept       json
// @Produce      json
// @Param        asset_id path int true "Asset ID"
// @Param        asset    body UpdateAssetRequest true "Fields to update"
// @Success      200 {object} AssetResponse
// @Failure      400 {object} ErrorResponse "Invalid input or asset ID"
// @Failure      404 {object} ErrorResponse "Asset not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/{asset_id} [put]
func (h *Handler) UpdateAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("asset_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid asset_id"))
		return
	}
	var req UpdateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.UpdateAsset(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== asset-set =====
// 将来的にcreateAssetMasterとCreateAssetを廃止してこっちへ移行．
// ただしAndroidとフロントエンドの対応が終わり次第移行すること．
// 20260411現在でまだ未対応なので旧実装のエンドポイント両方残している．

// @Summary      Create an asset set (master and instance)
// @Description  Creates an asset master and its asset instance in a single request for single registration flow.
// @Tags         assets-set
// @Accept       json
// @Produce      json
// @Param        assetSet body CreateAssetSetRequest true "Asset master and asset to create"
// @Success      201 {object} AssetSetResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      409 {object} ErrorResponse "Conflict while creating asset set"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/pair [post]
func (h *Handler) RegisterAsset(c *gin.Context) {
	var req CreateAssetSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateAssetSet: bind error: %v", err)
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAssetSet(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/assets/pair/"+res.Master.ManagementNumber)
	c.JSON(http.StatusCreated, res)
}

// @Summary      Get an asset set (master and instance)
// @Description  Get both master and instance details for an asset by its management number.
// @Tags         assets-set
// @Produce      json
// @Param        management_number path string true "Management Number"
// @Success      200 {object} AssetSetResponse
// @Failure      404 {object} ErrorResponse "Asset set not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/pair/{management_number} [get]
func (h *Handler) GetAssetSet(c *gin.Context) {
	mng := c.Param("management_number")
	res, err := h.svc.GetAssetSet(c.Request.Context(), mng)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== batch registration =====

// @Summary      Import assets from a CSV file
// @Description  Batch import assets by uploading a CSV file. The mode query parameter can be 'dry_run' or 'commit'.
// @Tags         assets-batch
// @Accept       multipart/form-data
// @Produce      json
// @Param        mode query string false "Import mode ('dry_run' or 'commit')" Enums(dry_run, commit) default(commit)
// @Param        file formData file true "CSV file to import"
// @Success      200 {object} ImportAssetsResponse
// @Failure      400 {object} ErrorResponse "Invalid mode or file"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/import [post]
func (h *Handler) HandleImportAssets(c *gin.Context) {
	// mode: dry_run | commit（デフォルト commit）
	mode := strings.ToLower(strings.TrimSpace(c.Query("mode")))
	if mode == "" {
		mode = "commit"
	}
	if mode != "dry_run" && mode != "commit" {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "mode must be 'dry_run' or 'commit'"))
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeNotFound, "file is required (multipart form field name: file)"))
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErr(CodeInternal, err.Error()))
		return
	}
	defer f.Close()

	// CSV reader
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	res, err := h.svc.ImportAssetsCSV(c.Request.Context(), r, mode)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// @Summary      Search assets with combined filters
// @Description  Searches joined asset master and asset rows with exact, partial, prefix, and range filters.
// @Description  `q` searches across management number, name, manufacturer, model, and serial.
// @Description  `management_number`, `asset_id`, `asset_master_id`, `genre_id`, `genre_code`, `management_category_id`, and `status_id` are exact-match filters.
// @Description  `management_number_prefix` is a prefix filter. Text filters such as `name`, `manufacturer`, `model`, `serial`, `owner`, `default_location`, `location`, `last_checked_by`, and `notes` use partial matches.
// @Description  `*_to` date filters accept `YYYY-MM-DD` or RFC3339. A date-only `*_to` value is treated as an inclusive day by converting it to the next UTC day internally.
// @Description  Examples:
// @Description  - GET /assets/search?management_number=OFS-20250901-0001
// @Description  - GET /assets/search?genre_id=10&status_id=1
// @Description  - GET /assets/search?q=ThinkPad
// @Description  - GET /assets/search?manufacturer=Lenovo&model=X1
// @Description  - GET /assets/search?created_from=2026-01-01&created_to=2026-03-31
// @Tags         assets-search
// @Produce      json
// @Param        q                        query string false "Cross-field partial search on management_number, name, manufacturer, model, and serial"
// @Param        management_number        query string false "Exact match on management number"
// @Param        management_number_prefix query string false "Prefix match on management number"
// @Param        asset_id                 query int    false "Exact match on asset ID"
// @Param        asset_master_id          query int    false "Exact match on asset master ID"
// @Param        genre_id                 query int    false "Exact match on genre ID"
// @Param        genre_code               query string false "Exact match on genre code"
// @Param        genre_name               query string false "Partial match on genre name"
// @Param        management_category_id   query int    false "Exact match on management category ID"
// @Param        name                     query string false "Partial match on asset master name"
// @Param        manufacturer             query string false "Partial match on manufacturer"
// @Param        model                    query string false "Partial match on model"
// @Param        serial                   query string false "Partial match on serial"
// @Param        status_id                query int    false "Exact match on asset status ID"
// @Param        owner                    query string false "Partial match on owner"
// @Param        default_location         query string false "Partial match on default location"
// @Param        location                 query string false "Partial match on current location"
// @Param        purchased_from           query string false "Purchased date lower bound (`YYYY-MM-DD` or RFC3339)" Format(date)
// @Param        purchased_to             query string false "Purchased date upper bound (`YYYY-MM-DD` or RFC3339); date-only values include the entire day" Format(date)
// @Param        created_from             query string false "Created date lower bound (`YYYY-MM-DD` or RFC3339)" Format(date)
// @Param        created_to               query string false "Created date upper bound (`YYYY-MM-DD` or RFC3339); date-only values include the entire day" Format(date)
// @Param        last_checked_from        query string false "Last checked date lower bound (`YYYY-MM-DD` or RFC3339)" Format(date)
// @Param        last_checked_to          query string false "Last checked date upper bound (`YYYY-MM-DD` or RFC3339); date-only values include the entire day" Format(date)
// @Param        last_checked_by          query string false "Partial match on last checked by"
// @Param        quantity_min             query int    false "Minimum quantity"
// @Param        quantity_max             query int    false "Maximum quantity"
// @Param        notes                    query string false "Partial match on notes"
// @Success      200 {array} AssetSetResponse
// @Failure      400 {object} ErrorResponse "Invalid query parameter"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/search [get]
func (h *Handler) SearchAssets(c *gin.Context) {
	// /assets/search は master と asset を結合して返すため、両方の項目を同時に受け付ける。
	// 文字列検索は exact/prefix と明示したもの以外を部分一致に寄せ、画面側の検索条件を増やしやすくしている。
	q, err := buildAssetSearchQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, err.Error()))
		return
	}

	res, err := h.svc.SearchAssets(c.Request.Context(), q)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ==== JANコード検索 ====

// @Summary      Lookup product info by JAN/ISBN code
// @Description  Fetches product name and manufacturer from external APIs using a JAN or ISBN code.
// @Tags         assets-lookup
// @Produce      json
// @Param        jan_code path string true "JAN or ISBN code"
// @Success      200 {object} JANLookupResponse
// @Failure      400 {object} ErrorResponse "JAN code is required"
// @Failure      404 {object} ErrorResponse "Product not found"
// @Failure      500 {object} ErrorResponse "Internal server error or external API error"
// @Router       /assets/lookup/{jan_code} [get]
func (h *Handler) LookupJAN(c *gin.Context) {
	jan := c.Param("jan_code")
	if jan == "" {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "jan_code is required"))
		return
	}

	res, err := h.svc.LookupJAN(c.Request.Context(), jan)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== helpers =====

func atoiDef(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}

func buildAssetSearchQuery(c *gin.Context) (AssetSearchQuery, error) {
	var q AssetSearchQuery

	q.Q = trimmedQueryValue(c, "q")
	q.ManagementNumber = trimmedQueryValue(c, "management_number")
	q.ManagementNumberPrefix = trimmedQueryValue(c, "management_number_prefix")
	q.GenreCode = trimmedQueryValue(c, "genre_code")
	q.GenreName = trimmedQueryValue(c, "genre_name")
	q.Name = trimmedQueryValue(c, "name")
	q.Manufacturer = trimmedQueryValue(c, "manufacturer")
	q.Model = trimmedQueryValue(c, "model")
	q.Serial = trimmedQueryValue(c, "serial")
	q.Owner = trimmedQueryValue(c, "owner")
	q.DefaultLocation = trimmedQueryValue(c, "default_location")
	q.Location = trimmedQueryValue(c, "location")
	q.LastCheckedBy = trimmedQueryValue(c, "last_checked_by")
	q.Notes = trimmedQueryValue(c, "notes")

	var err error
	if q.AssetID, err = parseOptionalUint64Query(c, "asset_id"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.AssetMasterID, err = parseOptionalUint64Query(c, "asset_master_id"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.GenreID, err = parseOptionalUintQuery(c, "genre_id"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.ManagementCategoryID, err = parseOptionalUintQuery(c, "management_category_id"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.StatusID, err = parseOptionalUintQuery(c, "status_id"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.QuantityMin, err = parseOptionalUintQuery(c, "quantity_min"); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.QuantityMax, err = parseOptionalUintQuery(c, "quantity_max"); err != nil {
		return AssetSearchQuery{}, err
	}

	// *_to は日付のみが渡された場合に「その日を含む上限」として扱えるよう、
	// 次の UTC 日付へ丸めてから SQL 側で半開区間 (<) にする。
	if q.PurchasedFrom, err = parseOptionalTimeQuery(c, "purchased_from", false); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.PurchasedTo, err = parseOptionalTimeQuery(c, "purchased_to", true); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.CreatedFrom, err = parseOptionalTimeQuery(c, "created_from", false); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.CreatedTo, err = parseOptionalTimeQuery(c, "created_to", true); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.LastCheckedFrom, err = parseOptionalTimeQuery(c, "last_checked_from", false); err != nil {
		return AssetSearchQuery{}, err
	}
	if q.LastCheckedTo, err = parseOptionalTimeQuery(c, "last_checked_to", true); err != nil {
		return AssetSearchQuery{}, err
	}

	return q, nil
}

func trimmedQueryValue(c *gin.Context, key string) *string {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return nil
	}
	return &v
}

func parseOptionalUint64Query(c *gin.Context, key string) (*uint64, error) {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return nil, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be uint64", key)
	}
	return &n, nil
}

func parseOptionalUintQuery(c *gin.Context, key string) (*uint, error) {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return nil, nil
	}
	n, err := parseUint(v)
	if err != nil {
		return nil, fmt.Errorf("%s must be uint", key)
	}
	return &n, nil
}

func parseOptionalTimeQuery(c *gin.Context, key string, endExclusive bool) (*time.Time, error) {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return nil, nil
	}
	t, err := parseSearchTimeQueryValue(key, v, endExclusive)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseSearchTimeQueryValue(key, value string, endExclusive bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}

	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD or RFC3339", key)
	}
	if endExclusive {
		t = t.AddDate(0, 0, 1)
	}
	return t.UTC(), nil
}

func nextOffset(total int64, p Page) int {
	n := p.Offset + p.Limit
	if n >= int(total) {
		return 0
	}
	return n
}

type errDTO struct {
	Error struct {
		Code    Code   `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func apiErr(code Code, msg string) errDTO {
	var e errDTO
	e.Error.Code = code
	e.Error.Message = msg
	return e
}
func apiErrFrom(err error) errDTO {
	if api, ok := err.(*APIError); ok {
		return apiErr(api.Code, api.Message)
	}
	return apiErr(CodeInternal, err.Error())
}
