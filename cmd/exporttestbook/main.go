// Export a read-only snapshot of local test data. Run from Backend.
package main

import (
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"sort"
	"student-management/models"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	_ = godotenv.Load(".env")
	cfg := driver.NewConfig()
	cfg.User = os.Getenv("DB_USER")
	if cfg.User == "" {
		cfg.User = "root"
	}
	cfg.Passwd = os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	cfg.Net = "tcp"
	cfg.Addr = host + ":" + port
	cfg.DBName = os.Getenv("DB_NAME")
	if cfg.DBName == "" {
		cfg.DBName = "student_manager_db"
	}
	cfg.ParseTime = true
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic("Không kết nối được database")
	}
	var users []models.User
	must(db.Preload("Role").Order("role_id, username").Find(&users).Error)
	var offerings []models.CourseOffering
	must(db.Preload("Class.Major").Preload("Course").Preload("Semester").Preload("Teacher.User").Preload("Room").Preload("Schedules").Order("class_id, semester_id, course_id").Find(&offerings).Error)
	var students []models.Student
	must(db.Preload("User").Preload("Class").Preload("Major").Order("class_id, student_code").Find(&students).Error)
	var enrollments []models.Enrollment
	must(db.Where("status <> ?", "cancelled").Order("course_offering_id, student_id").Find(&enrollments).Error)
	byStudent := map[uint]models.Student{}
	for _, s := range students {
		byStudent[s.ID] = s
	}
	byOffering := map[uint]models.CourseOffering{}
	counts := map[uint]int{}
	for _, o := range offerings {
		byOffering[o.ID] = o
	}
	for _, e := range enrollments {
		if e.CourseOfferingID != nil {
			counts[*e.CourseOfferingID]++
		}
	}
	passwords := map[uint]string{}
	verified := 0
	for _, u := range users {
		passwords[u.ID] = "Chưa xác minh - dùng mật khẩu đã đổi"
		if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("123456")) == nil {
			passwords[u.ID] = "123456"
			verified++
		}
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	if loc == nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Huong dan")
	header, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"2447A8"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center", WrapText: true}})
	must(err)
	body, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 11}, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	must(err)
	sheet := func(name string, head []interface{}, rows [][]interface{}) {
		if name != "Huong dan" {
			_, err = f.NewSheet(name)
			must(err)
		}
		must(f.SetSheetRow(name, "A1", &head))
		for i, r := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, i+2)
			must(f.SetSheetRow(name, cell, &r))
		}
		last, _ := excelize.ColumnNumberToName(len(head))
		must(f.SetColWidth(name, "A", last, 24))
		must(f.SetRowHeight(name, 1, 36))
		must(f.SetCellStyle(name, "A1", last+"1", header))
		if len(rows) > 0 {
			must(f.SetCellStyle(name, "A2", fmt.Sprintf("%s%d", last, len(rows)+1), body))
			must(f.AutoFilter(name, fmt.Sprintf("A1:%s%d", last, len(rows)+1), nil))
		}
		must(f.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}))
	}
	sheet("Huong dan", []interface{}{"Mục", "Hướng dẫn"}, [][]interface{}{
		{"Ngày xuất", now.Format("02/01/2006 15:04:05") + " (UTC+7)"}, {"Nguồn", "Snapshot database hiện tại; không thay đổi dữ liệu hay mật khẩu."}, {"Đăng nhập", "Có thể nhập username hoặc email. Mật khẩu 123456 chỉ được ghi khi kiểm tra bcrypt thành công. Tài khoản đã đổi mật khẩu được đánh dấu chưa xác minh."}, {"Bắt đầu test", "Lọc sheet Hom nay va sap toi theo ngày/lớp. Lấy mã học phần LHP, tìm cùng mã trong SV theo hoc phan để chọn sinh viên thực sự đăng ký môn đó."}, {"Điểm danh QR", "Phải đúng ngày học; giảng viên mở từ 15 phút trước giờ bắt đầu đến lúc kết thúc. QR hết hạn sau 1 phút. Chọn lớp có sinh viên trong sheet SV theo hoc phan."}, {"Tạo bài tập", "Dùng giảng viên được giao đúng học phần còn mở và còn lịch dạy. Hạn nộp ở tương lai. Sinh viên phải đăng ký học phần đó."}, {"Lịch quá khứ", "K24_WEB_01 có lịch kết thúc trước ngày xuất; dùng để kiểm tra khóa tạo bài tập và xem lịch sử. Không kỳ vọng có lịch hôm nay."}, {"Lịch định kỳ", "Sheet Lich dinh ky chứa lịch không có ngày cụ thể; đối chiếu thứ với khoảng ngày học kỳ."}, {"Checklist", "Điền Đạt/Không đạt và lỗi thực tế vào hai cột cuối. File này chưa xác nhận chức năng đã chạy đúng."}, {"Tài khoản", fmt.Sprintf("%d tài khoản; %d xác minh mật khẩu 123456; còn lại không suy đoán mật khẩu.", len(users), verified)},
	})
	accountRows := [][]interface{}{}
	for _, u := range users {
		accountRows = append(accountRows, []interface{}{u.Role.Name, u.FullName, u.Username, u.Email, passwords[u.ID], u.IsActive, u.FirstLogin})
	}
	sheet("Tai khoan", []interface{}{"Vai trò", "Họ tên", "Username", "Email đăng nhập", "Mật khẩu đã xác minh", "Đang hoạt động", "Lần đầu đăng nhập"}, accountRows)
	studentRows := [][]interface{}{}
	for _, s := range students {
		studentRows = append(studentRows, []interface{}{s.StudentCode, s.User.FullName, s.Class.ClassCode, s.Major.Name, s.User.Username, s.User.Email, passwords[s.UserID], s.Status})
	}
	sheet("Sinh vien", []interface{}{"Mã sinh viên", "Họ tên", "Lớp chủ nhiệm", "Chuyên ngành", "Username", "Email", "Mật khẩu", "Trạng thái SV"}, studentRows)
	roster := [][]interface{}{}
	for _, e := range enrollments {
		if e.CourseOfferingID == nil {
			continue
		}
		o := byOffering[*e.CourseOfferingID]
		s := byStudent[e.StudentID]
		roster = append(roster, []interface{}{o.OfferingCode, o.Class.ClassCode, o.Course.Name, o.Teacher.User.FullName, s.StudentCode, s.User.FullName, s.User.Username, s.User.Email, passwords[s.UserID], e.Status})
	}
	sheet("SV theo hoc phan", []interface{}{"Mã học phần", "Lớp học", "Môn học", "Giảng viên", "Mã sinh viên", "Sinh viên", "Username SV", "Email SV", "Mật khẩu SV", "Trạng thái đăng ký"}, roster)
	type lesson struct {
		date string
		row  []interface{}
	}
	lessons := []lesson{}
	recurring := [][]interface{}{}
	offRows := [][]interface{}{}
	days := []string{"Chủ nhật", "Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy"}
	for _, o := range offerings {
		first, last := "", ""
		for _, s := range o.Schedules {
			if s.FullDate == nil {
				recurring = append(recurring, []interface{}{o.OfferingCode, o.Class.ClassCode, o.Course.Name, s.DayOfWeek, s.StartTime, s.EndTime, o.Semester.StartDate.Format("2006-01-02"), o.Semester.EndDate.Format("2006-01-02"), o.Teacher.User.FullName, o.Teacher.User.Email, o.Room.Name})
				continue
			}
			date := s.FullDate.Format("2006-01-02")
			if first == "" || date < first {
				first = date
			}
			if date > last {
				last = date
			}
			state := "Sắp tới"
			if date < today {
				state = "Đã qua"
			} else if date == today {
				state = "Hôm nay"
			}
			r := []interface{}{date, days[int(s.FullDate.Weekday())], s.StartTime, s.EndTime, o.Class.ClassCode, o.Course.Name, o.OfferingCode, o.Room.Name, o.Teacher.User.FullName, o.Teacher.User.Username, o.Teacher.User.Email, passwords[o.Teacher.UserID], counts[o.ID], o.Status, state}
			lessons = append(lessons, lesson{date + " " + s.StartTime, r})
		}
		offRows = append(offRows, []interface{}{o.OfferingCode, o.Class.ClassCode, o.Class.Major.Name, o.Course.Name, o.Semester.Name, o.Status, first, last, o.Teacher.User.FullName, o.Teacher.User.Username, o.Teacher.User.Email, passwords[o.Teacher.UserID], counts[o.ID]})
	}
	sort.SliceStable(lessons, func(i, j int) bool { return lessons[i].date < lessons[j].date })
	all, upcoming := [][]interface{}{}, [][]interface{}{}
	for _, l := range lessons {
		all = append(all, l.row)
		if l.date[:10] >= today {
			upcoming = append(upcoming, l.row)
		}
	}
	heads := []interface{}{"Ngày (YYYY-MM-DD)", "Thứ", "Giờ bắt đầu", "Giờ kết thúc", "Lớp", "Môn học", "Mã học phần", "Phòng", "Giảng viên", "Username GV", "Email GV", "Mật khẩu GV", "Số SV đăng ký", "Trạng thái học phần", "Thời điểm"}
	sheet("Hom nay va sap toi", heads, upcoming)
	sheet("Toan bo lich day", heads, all)
	sheet("Hoc phan", []interface{}{"Mã học phần", "Lớp", "Chuyên ngành", "Môn học", "Học kỳ", "Trạng thái", "Buổi đầu", "Buổi cuối", "Giảng viên", "Username GV", "Email GV", "Mật khẩu GV", "Số SV đăng ký"}, offRows)
	sheet("Lich dinh ky", []interface{}{"Mã học phần", "Lớp", "Môn", "Thứ trong tuần", "Bắt đầu", "Kết thúc", "Đầu học kỳ", "Cuối học kỳ", "Giảng viên", "Email GV", "Phòng"}, recurring)
	tests := [][]interface{}{
		{"01", "Chung", "Đăng nhập / đăng xuất", "Dùng tài khoản từng vai trò tại Tai khoan; thử email, username và mật khẩu sai.", "Vào đúng vai trò; sai mật khẩu báo lỗi; đăng xuất không truy cập trang riêng."},
		{"02", "Sinh viên", "Hồ sơ / trang chủ", "Chọn SV tại Sinh vien; đăng nhập web/app.", "Đúng tên, mã SV, lớp, chuyên ngành."},
		{"03", "GV / SV", "Lịch ngày, tuần, tháng", "Chọn ngày có lịch trong Hom nay va sap toi; đối chiếu cùng mã học phần.", "Đúng thứ, ngày, giờ, phòng, môn và GV; SV chỉ thấy môn đã đăng ký."},
		{"04", "GV", "Tạo bài tập", "Dùng GV tại Hoc phan có status open và buổi cuối ở tương lai; nhập hạn nộp tương lai.", "Lưu được; bài mới hiện ngay; SV cùng học phần thấy bài."},
		{"05", "GV", "Chặn tạo bài đã học xong", "Chọn học phần K24 đã hết lịch.", "Khóa môn đã kết thúc; không khóa môn còn lịch dạy."},
		{"06", "SV / GV", "Nộp bài / chấm điểm", "SV trong SV theo hoc phan nộp bài; GV xem, chấm; SV xem lại.", "Đúng bài nộp, đúng người; điểm và phản hồi đồng bộ."},
		{"07", "GV / SV", "Khóa, gia hạn, nộp trễ", "GV đóng bài rồi mở lại hạn mới; SV thử nộp ở cả hai trạng thái.", "Chặn khi đóng/quá hạn; cho nộp khi đã mở và còn hạn."},
		{"08", "GV / SV", "Điểm danh QR", "Đúng ngày giờ lịch: GV mở QR; SV cùng học phần quét; thử quét lại, sai lớp và mã hết hạn.", "Ghi nhận một lượt hợp lệ; chặn sai lớp, mã cũ và trùng lượt."},
		{"09", "Admin / GV", "Phân công / lời mời", "Admin gửi lịch học không trùng cho GV; GV chấp nhận hoặc từ chối.", "Lời mời mới hiện ngay, mới nhất trên đầu; chi tiết mở popup; phản hồi đồng bộ."},
		{"10", "Admin / GV / SV", "Thông báo", "Gửi cho một lớp; đăng nhập GV và SV liên quan.", "Đúng lớp nhận; không lặp bản ghi trên danh sách; mở xem đủ nội dung."},
		{"11", "Admin", "Chuyên ngành", "Tạo chuyên ngành mới bằng tên; xem form sinh viên/lớp/môn trên web và app.", "Mã tự sinh chữ hoa; danh mục đồng bộ; chỉ chọn ngành trường đang đào tạo."},
		{"12", "Admin", "Sinh viên / giảng viên", "Tạo, sửa thông tin với dữ liệu test riêng; thử email/mã trùng.", "Dữ liệu hợp lệ được lưu; trùng bị chặn; danh sách cập nhật."},
		{"13", "Admin", "Lớp / phòng / môn / học kỳ", "Tạo dữ liệu test riêng; kiểm tra liên kết chuyên ngành và điều kiện lịch.", "Đúng danh mục liên quan; kiểm tra xung đột phòng/GV/lớp."},
		{"14", "SV / Admin", "Đăng ký học phần", "SV đăng ký môn còn mở đúng điều kiện; admin xử lý nếu có bước duyệt.", "Không trùng đăng ký; lịch và danh sách lớp cập nhật sau khi được nhận."},
		{"15", "GV / Admin / SV", "Điểm / duyệt điểm / bảng điểm", "GV nhập điểm; admin duyệt; SV kiểm tra môn tương ứng.", "Điểm, học kỳ và kết quả khớp; không lẫn lớp/học phần."},
		{"16", "Admin", "Báo cáo / cảnh báo học vụ", "Đối chiếu dữ liệu điểm, điểm danh và bộ lọc học kỳ/lớp.", "Số liệu khớp; cảnh báo đến đúng người; xuất báo cáo dùng được."},
		{"17", "Tất cả", "Phân trang / tìm kiếm / lọc", "Mở từng danh sách web/app có hơn 20 dòng; chuyển trang, tìm và lọc.", "Tối đa 20 dòng/trang; không mất/trùng dòng; bộ lọc đưa về trang phù hợp."},
		{"18", "SV / GV", "Đổi mật khẩu", "Dùng tài khoản test đã chọn; đổi rồi đăng nhập lại.", "Mật khẩu mới hoạt động; ghi lại riêng vì file snapshot không tự cập nhật."},
		{"19", "App", "Animation / tải lại / lỗi mạng", "Chuyển tab, popup, làm mới; thử khi mất mạng.", "Thao tác mượt; không gửi trùng; báo lỗi rõ; thử lại được."},
	}
	for i := range tests {
		tests[i] = append(tests[i], "Chưa test", "")
	}
	sheet("Checklist test", []interface{}{"STT", "Vai trò", "Chức năng", "Cách test", "Kết quả mong đợi", "Đạt / Không đạt", "Lỗi thực tế / ghi chú"}, tests)
	must(f.SetColWidth("Huong dan", "B", "B", 110))
	must(f.SetColWidth("Checklist test", "D", "G", 55))
	f.SetActiveSheet(0)
	output := "../artifacts/PP_Academy_Du_lieu_Test.xlsx"
	must(os.MkdirAll("../artifacts", 0755))
	must(f.SaveAs(output))
	fmt.Printf("Exported %s: %d users (%d verified passwords), %d students, %d enrollments, %d dated lessons, %d upcoming/today.\n", output, len(users), verified, len(students), len(enrollments), len(all), len(upcoming))
}
