package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"student-management/config"
	"student-management/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type siteSettingRequest struct {
	SchoolName          string `json:"schoolName"`
	Slogan              string `json:"slogan"`
	Introduction        string `json:"introduction"`
	HeroImageURL        string `json:"heroImageUrl"`
	LogoURL             string `json:"logoUrl"`
	DownloadButtonLabel string `json:"downloadButtonLabel"`
	DownloadURL         string `json:"downloadUrl"`
	DownloadEnabled     bool   `json:"downloadEnabled"`
	ContactEmail        string `json:"contactEmail"`
	Phone               string `json:"phone"`
	Address             string `json:"address"`
}

type schoolPostRequest struct {
	Title         string `json:"title"`
	Excerpt       string `json:"excerpt"`
	Content       string `json:"content"`
	CoverImageURL string `json:"coverImageUrl"`
	AuthorName    string `json:"authorName"`
	IsPublished   bool   `json:"isPublished"`
	IsFeatured    bool   `json:"isFeatured"`
}

func ensurePublicWebsiteDefaults() error {
	setting := models.SiteSetting{
		ID:                  1,
		SchoolName:          "PP Academy",
		Slogan:              "Thông tin đào tạo và học tập",
		Introduction:        "Trang thông tin PP Academy cung cấp thông báo, giới thiệu chương trình đào tạo và các tiện ích học tập cho sinh viên, giảng viên.",
		HeroImageURL:        "/pp-academy-campus.webp",
		LogoURL:             "/pp-academy-logo.svg",
		DownloadButtonLabel: "Tải ứng dụng",
		DownloadEnabled:     false,
		ContactEmail:        "hello@ppacademy.edu.vn",
		Phone:               "028 7300 6868",
		Address:             "Thành phố Hồ Chí Minh, Việt Nam",
	}
	if err := config.DB.FirstOrCreate(&setting, models.SiteSetting{ID: 1}).Error; err != nil {
		return err
	}

	var count int64
	if err := config.DB.Model(&models.SchoolPost{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now()
	posts := []models.SchoolPost{
		{
			Title:         "PP Academy mở rộng không gian học tập số",
			Slug:          "pp-academy-mo-rong-khong-gian-hoc-tap-so",
			Excerpt:       "Hệ sinh thái học tập mới kết nối lớp học, phòng thực hành và tài nguyên trực tuyến trong một trải nghiệm thống nhất.",
			Content:       "PP Academy chính thức đưa vào vận hành không gian học tập số dành cho sinh viên và giảng viên. Hệ thống hỗ trợ quản lý lớp học, học liệu, bài tập và các hoạt động học thuật trên cùng một nền tảng.\n\nBên cạnh hạ tầng công nghệ, học viện tiếp tục đầu tư các phòng thực hành theo hướng dự án để sinh viên được trải nghiệm quy trình làm việc thực tế ngay trong quá trình học.\n\nĐây là một phần trong định hướng xây dựng môi trường học tập linh hoạt, hiện đại và lấy người học làm trung tâm của PP Academy.",
			CoverImageURL: "/pp-academy-tech-lab.webp", AuthorName: "Ban Truyền thông", IsPublished: true, IsFeatured: true, PublishedAt: timePointer(now.AddDate(0, 0, -2)),
		},
		{
			Title:         "Tuần lễ kết nối sinh viên và doanh nghiệp",
			Slug:          "tuan-le-ket-noi-sinh-vien-va-doanh-nghiep",
			Excerpt:       "Chuỗi hoạt động định hướng nghề nghiệp giúp sinh viên tiếp cận doanh nghiệp, chuyên gia và cơ hội thực tập phù hợp.",
			Content:       "Tuần lễ kết nối sinh viên và doanh nghiệp tại PP Academy mang đến các phiên tư vấn nghề nghiệp, workshop kỹ năng và không gian trao đổi trực tiếp với nhà tuyển dụng.\n\nSinh viên được hướng dẫn hoàn thiện hồ sơ cá nhân, thực hành phỏng vấn và tìm hiểu yêu cầu thực tế của thị trường lao động. Các hoạt động được thiết kế theo từng nhóm ngành để bảo đảm tính thiết thực và khả năng ứng dụng.\n\nPP Academy xem sự đồng hành của doanh nghiệp là một phần quan trọng trong quá trình đào tạo và phát triển nghề nghiệp cho người học.",
			CoverImageURL: "/pp-academy-campus.webp", AuthorName: "Phòng Công tác sinh viên", IsPublished: true, PublishedAt: timePointer(now.AddDate(0, 0, -6)),
		},
		{
			Title:         "Thư viện mở rộng khu học tập nhóm",
			Slug:          "thu-vien-mo-rong-khu-hoc-tap-nhom",
			Excerpt:       "Không gian mới tạo điều kiện cho sinh viên nghiên cứu, thảo luận và triển khai các dự án liên ngành.",
			Content:       "Khu học tập nhóm mới tại thư viện PP Academy được thiết kế theo hướng mở, linh hoạt và thuận tiện cho các hoạt động nghiên cứu.\n\nKhông gian cung cấp khu vực thảo luận, hệ thống trình chiếu, nguồn học liệu số và các bàn làm việc phù hợp với nhiều quy mô nhóm. Sinh viên có thể đặt lịch sử dụng và chủ động tổ chức các phiên học tập theo dự án.\n\nViệc mở rộng thư viện tiếp tục khẳng định cam kết của học viện trong việc tạo dựng môi trường học tập tích cực và giàu tính kết nối.",
			CoverImageURL: "/pp-academy-library.webp", AuthorName: "Thư viện PP Academy", IsPublished: true, PublishedAt: timePointer(now.AddDate(0, 0, -10)),
		},
	}
	return config.DB.Create(&posts).Error
}

func timePointer(value time.Time) *time.Time { return &value }

func GetPublicHome(c *gin.Context) {
	if err := ensurePublicWebsiteDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được trang chủ", "error": err.Error()})
		return
	}
	var setting models.SiteSetting
	var posts []models.SchoolPost
	if err := config.DB.First(&setting, 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được cấu hình trang chủ"})
		return
	}
	if err := config.DB.Where("is_published = ?", true).Order("is_featured desc, published_at desc, id desc").Limit(6).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy nội dung trang chủ thành công", "data": gin.H{"settings": setting, "posts": posts}})
}

func GetPublicPosts(c *gin.Context) {
	if err := ensurePublicWebsiteDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết"})
		return
	}
	var posts []models.SchoolPost
	if err := config.DB.Where("is_published = ?", true).Order("published_at desc, id desc").Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài viết thành công", "data": posts})
}

func GetPublicPost(c *gin.Context) {
	var post models.SchoolPost
	if err := config.DB.Where("slug = ? AND is_published = ?", strings.TrimSpace(c.Param("slug")), true).First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bài viết"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy bài viết thành công", "data": post})
}

func GetAdminSiteSettings(c *gin.Context) {
	if err := ensurePublicWebsiteDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được cấu hình trang chủ"})
		return
	}
	var setting models.SiteSetting
	if err := config.DB.First(&setting, 1).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được cấu hình trang chủ"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy cấu hình thành công", "data": setting})
}

func UpdateAdminSiteSettings(c *gin.Context) {
	var req siteSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu cấu hình không hợp lệ"})
		return
	}
	if strings.TrimSpace(req.SchoolName) == "" || strings.TrimSpace(req.Introduction) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Tên trường và nội dung giới thiệu không được để trống"})
		return
	}
	for label, value := range map[string]string{"ảnh banner": req.HeroImageURL, "logo": req.LogoURL, "link tải ứng dụng": req.DownloadURL} {
		if !validWebsiteURL(value) {
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("%s không hợp lệ", label)})
			return
		}
	}
	if req.DownloadEnabled && strings.TrimSpace(req.DownloadURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Cần nhập link tải trước khi bật nút tải ứng dụng"})
		return
	}
	if err := ensurePublicWebsiteDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không lưu được cấu hình"})
		return
	}
	updates := map[string]interface{}{
		"school_name": strings.TrimSpace(req.SchoolName), "slogan": strings.TrimSpace(req.Slogan), "introduction": strings.TrimSpace(req.Introduction),
		"hero_image_url": strings.TrimSpace(req.HeroImageURL), "logo_url": strings.TrimSpace(req.LogoURL), "download_button_label": strings.TrimSpace(req.DownloadButtonLabel),
		"download_url": strings.TrimSpace(req.DownloadURL), "download_enabled": req.DownloadEnabled, "contact_email": strings.TrimSpace(req.ContactEmail),
		"phone": strings.TrimSpace(req.Phone), "address": strings.TrimSpace(req.Address),
	}
	if err := config.DB.Model(&models.SiteSetting{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không lưu được cấu hình", "error": err.Error()})
		return
	}
	GetAdminSiteSettings(c)
}

func ListAdminPosts(c *gin.Context) {
	if err := ensurePublicWebsiteDefaults(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết"})
		return
	}
	var posts []models.SchoolPost
	if err := config.DB.Order("created_at desc, id desc").Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không tải được bài viết", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Lấy danh sách bài viết thành công", "data": posts})
}

func CreateAdminPost(c *gin.Context) {
	var req schoolPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu bài viết không hợp lệ"})
		return
	}
	if message := validatePostRequest(req); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": message})
		return
	}
	now := time.Now()
	post := models.SchoolPost{
		Title: strings.TrimSpace(req.Title), Slug: uniquePostSlug(slugify(req.Title), 0), Excerpt: strings.TrimSpace(req.Excerpt),
		Content: strings.TrimSpace(req.Content), CoverImageURL: strings.TrimSpace(req.CoverImageURL), AuthorName: defaultAuthor(req.AuthorName),
		IsPublished: req.IsPublished, IsFeatured: req.IsFeatured,
	}
	if post.IsPublished {
		post.PublishedAt = &now
	}
	if err := config.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không tạo được bài viết", "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Đã tạo bài viết", "data": post})
}

func UpdateAdminPost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID bài viết không hợp lệ"})
		return
	}
	var post models.SchoolPost
	if err := config.DB.First(&post, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bài viết"})
		return
	}
	var req schoolPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Dữ liệu bài viết không hợp lệ"})
		return
	}
	if message := validatePostRequest(req); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": message})
		return
	}
	if req.IsPublished && post.PublishedAt == nil {
		now := time.Now()
		post.PublishedAt = &now
	}
	if !req.IsPublished {
		post.PublishedAt = nil
	}
	post.Title = strings.TrimSpace(req.Title)
	post.Slug = uniquePostSlug(slugify(req.Title), post.ID)
	post.Excerpt = strings.TrimSpace(req.Excerpt)
	post.Content = strings.TrimSpace(req.Content)
	post.CoverImageURL = strings.TrimSpace(req.CoverImageURL)
	post.AuthorName = defaultAuthor(req.AuthorName)
	post.IsPublished = req.IsPublished
	post.IsFeatured = req.IsFeatured
	if err := config.DB.Save(&post).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Không cập nhật được bài viết", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã cập nhật bài viết", "data": post})
}

func DeleteAdminPost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "ID bài viết không hợp lệ"})
		return
	}
	result := config.DB.Delete(&models.SchoolPost{}, uint(id))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Không xóa được bài viết", "error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "Không tìm thấy bài viết"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa bài viết"})
}

func validatePostRequest(req schoolPostRequest) string {
	if strings.TrimSpace(req.Title) == "" {
		return "Tiêu đề không được để trống"
	}
	if strings.TrimSpace(req.Content) == "" {
		return "Nội dung không được để trống"
	}
	if !validWebsiteURL(req.CoverImageURL) {
		return "Link ảnh đại diện không hợp lệ"
	}
	return ""
}

func defaultAuthor(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "Ban Truyền thông PP Academy"
}

func validWebsiteURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	replacements := map[rune]rune{'đ': 'd', 'Đ': 'd'}
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if replacement, ok := replacements[r]; ok {
			builder.WriteRune(replacement)
			continue
		}
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		decomposed := removeVietnameseAccent(r)
		builder.WriteRune(decomposed)
	}
	slug := strings.Trim(slugInvalid.ReplaceAllString(builder.String(), "-"), "-")
	if slug == "" {
		return "bai-viet"
	}
	return slug
}

func removeVietnameseAccent(r rune) rune {
	if strings.ContainsRune("áàảãạăắằẳẵặâấầẩẫậ", r) {
		return 'a'
	}
	if strings.ContainsRune("éèẻẽẹêếềểễệ", r) {
		return 'e'
	}
	if strings.ContainsRune("íìỉĩị", r) {
		return 'i'
	}
	if strings.ContainsRune("óòỏõọôốồổỗộơớờởỡợ", r) {
		return 'o'
	}
	if strings.ContainsRune("úùủũụưứừửữự", r) {
		return 'u'
	}
	if strings.ContainsRune("ýỳỷỹỵ", r) {
		return 'y'
	}
	return r
}

func uniquePostSlug(base string, excludeID uint) string {
	candidate := base
	for suffix := 2; ; suffix++ {
		var count int64
		query := config.DB.Model(&models.SchoolPost{}).Where("slug = ?", candidate)
		if excludeID > 0 {
			query = query.Where("id <> ?", excludeID)
		}
		if query.Count(&count).Error == nil && count == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
}
