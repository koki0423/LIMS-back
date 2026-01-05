package dbmng

type CreateGenreRequest struct {
	GenreName string `json:"name" binding:"required"`
	GenreCode string `json:"code" binding:"required"`
}

type UpdateGenreRequest struct {
	GenreName  string `json:"name" binding:"required"`
	GenreCode  string `json:"code" binding:"required"`
	IsDisabled bool   `json:"is_disabled"`
}

type AssetGenre struct {
	GenreID    uint   `gorm:"primaryKey;column:genre_id" json:"id"`
	GenreName  string `gorm:"column:genre_name"          json:"name"`
	GenreCode  string `gorm:"column:genre_code"          json:"code"`
	IsDisabled bool   `gorm:"column:is_disabled"         json:"is_disabled"` // ★変更
}
