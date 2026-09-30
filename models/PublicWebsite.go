package models

import "time"

// SiteSetting stores the public PP Academy landing-page configuration.
// The application uses a single row whose ID is always 1.
type SiteSetting struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	SchoolName          string    `gorm:"size:160;not null" json:"schoolName"`
	Slogan              string    `gorm:"size:255" json:"slogan"`
	Introduction        string    `gorm:"type:text" json:"introduction"`
	HeroImageURL        string    `gorm:"size:500" json:"heroImageUrl"`
	LogoURL             string    `gorm:"size:500" json:"logoUrl"`
	DownloadButtonLabel string    `gorm:"size:100" json:"downloadButtonLabel"`
	DownloadURL         string    `gorm:"size:1000" json:"downloadUrl"`
	DownloadEnabled     bool      `gorm:"default:false" json:"downloadEnabled"`
	ContactEmail        string    `gorm:"size:190" json:"contactEmail"`
	Phone               string    `gorm:"size:30" json:"phone"`
	Address             string    `gorm:"size:255" json:"address"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// SchoolPost is a news/article shown on the public PP Academy website.
type SchoolPost struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Title         string     `gorm:"size:255;not null" json:"title"`
	Slug          string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Excerpt       string     `gorm:"type:text" json:"excerpt"`
	Content       string     `gorm:"type:longtext;not null" json:"content"`
	CoverImageURL string     `gorm:"size:500" json:"coverImageUrl"`
	AuthorName    string     `gorm:"size:120" json:"authorName"`
	IsPublished   bool       `gorm:"default:true;index" json:"isPublished"`
	IsFeatured    bool       `gorm:"default:false;index" json:"isFeatured"`
	PublishedAt   *time.Time `gorm:"index" json:"publishedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}
