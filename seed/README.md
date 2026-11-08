# Dữ liệu mẫu PP Academy

Seed theo yêu cầu đồ án: 4 chuyên ngành, 120 môn (30 môn/ngành), 24 lớp,
20 giảng viên, 480 sinh viên (20 sinh viên/lớp) và 10 phòng có mã A101–E102.
K24, K25, K26 mỗi khóa có 8 lớp, mỗi ngành có 2 lớp/khóa.
10 phòng học là tài nguyên dùng chung, tách biệt với 24 lớp học.

Mỗi khóa có đúng 4 học kỳ, tên kèm khóa để phân biệt: HK1 - K24 đến HK4 - K24,
HK1 - K25 đến HK4 - K25, HK1 - K26 đến HK4 - K26. Không nối số giữa các khóa.
K24 học 09/2024–08/2026; K25 học 09/2025–08/2027; K26 học 09/2026–08/2028.
Tổng 12 bản ghi học kỳ cho 3 khóa, mỗi khóa 4 học kỳ. Không tạo ngoại lệ HK5.
Mỗi chương trình chia 8/8/7/7 môn; SemesterID của môn là mốc chương trình K24,
còn học kỳ thực tế của mỗi khóa nằm trong CourseOffering hoặc kế hoạch lời mời.
Mỗi môn có 3 buổi mẫu; các môn lần lượt học theo từng khối 3 tuần.
Lịch nền chia 3 ca và rải các ngày trong tuần. Riêng 8 lớp K26 học 13:30–16:30,
bắt đầu thứ Tư sắp tới đến hết HK4 (31/08/2028), theo nhóm ngày cố định:

| Lớp | Ngày học mỗi tuần |
|---|---|
| K26_DESIGN_01, K26_MKT_01 | Thứ 2, 4 |
| K26_GAME_01, K26_WEB_01 | Thứ 2, 4, 6 |
| K26_DESIGN_02, K26_MKT_02 | Thứ 3, 5, 7 |
| K26_GAME_02, K26_WEB_02 | Thứ 3, 5 |

Các buổi trong một khối 3 tuần thuộc cùng một môn; khối cuối học kỳ kéo dài
đến cuối học kỳ. Mỗi thứ Tư có 4 lớp thuộc đủ 4 chuyên ngành để demo điểm danh.
Các lớp K26 dùng phòng và giảng viên riêng trong giờ demo; giữ nguyên lịch đã học.
Chạy `go run ./cmd/k26demo` để cập nhật riêng lịch demo trên database hiện có.
Lệnh từ chối thay lịch nếu đã có phiên điểm danh trong khoảng cần cập nhật.
Mở điểm danh đúng ngày học, từ 13:15 đến 16:30; sinh viên chỉ điểm danh khi
giảng viên đã mở phiên. Ví dụ tên đăng nhập `teacher01` và `student321`
thuộc lớp K26_WEB_01, mật khẩu mẫu `123456`.
Dữ liệu có lời mời đã chấp nhận/đang chờ, đăng ký đã thanh toán và xếp lớp,
điểm danh đã chốt, điểm và bảng điểm cho môn hoàn tất. Môn chưa học không có điểm cuối kỳ.
Lịch chính thức không trùng lớp, giảng viên hoặc phòng. Lời mời thông thường
cách buổi đầu ít nhất 7 ngày. Không tạo phiên đăng nhập JWT sẵn.

## Reset trong môi trường phát triển

Chạy từ thư mục `Backend`, dùng cấu hình kết nối trong `.env`:

```powershell
# Chỉ xem database đích, không thay đổi dữ liệu
go run ./cmd/dbreset

# Thay tên database bằng đúng DB_NAME trong .env
go run ./cmd/dbreset -execute -database student_manager_db -mysql-bin 'C:\Program Files\MySQL\MySQL Server 9.2\bin'
```

Dừng backend trước khi reset để tránh ghi dữ liệu đồng thời. Lệnh sẽ kiểm tra seed
trên schema tạm, sao lưu SQL database hiện tại, sau đó thay dữ liệu và kiểm tra lại.
Nếu thay dữ liệu thất bại, lệnh thử khôi phục bản sao lưu tự động. Không dùng cho production.

Mặc định lịch được tạo theo ngày chạy; dùng `-date 2026-11-02` để tái tạo cùng mốc.
`SeedData()` chỉ bổ sung vào database rỗng, từ chối ghi đè tài khoản hiện có.

## Tài khoản và bản sao lưu

Thư mục `Backend/backups/` được Git bỏ qua, chứa bản SQL cũ, SQL seed mới,
manifest có SHA-256 và file `accounts_*.txt` ghi mật khẩu mẫu mặc định `123456`.
Không đưa thư mục này lên Git hoặc chia sẻ công khai: bản sao lưu chứa dữ liệu cũ.
Có thể đặt `SEED_PASSWORD` (6–72 byte) trước khi chạy nếu cần mật khẩu riêng.

Toàn bộ mã tạo dữ liệu nằm trong `seed/Seeding.go`: khởi tạo seed, chương trình
30 môn mỗi ngành, học kỳ, lớp, sinh viên, giảng viên và kiểm tra dữ liệu.
Các file `_test.go` chỉ dùng để kiểm thử.

- Quản trị viên: `admin`.
- Giảng viên: `teacher01` đến `teacher20`.
- Sinh viên: `student001` đến `student480`.
- Mã sinh viên: năm nhập học + hệ (`CQ`, `PLUS`, `THS`, `TS`) + mã ngành + STT 5 chữ số.
  Email: họ tên viết tắt không dấu + mã ngành + STT, dưới miền `ppacademy.edu.vn`.
  Ví dụ Nguyễn Minh Anh: `2026CQWEB00481`, `nma.web00481@ppacademy.edu.vn`, `student481`.
- `teacher04` và `student012` minh họa luồng đổi mật khẩu lần đầu.

Sau reset cần đăng nhập lại vì phiên đăng nhập cũ đã bị xóa.

## Kiểm tra

```powershell
$env:SEED_INTEGRATION='1'
go test ./... -count=1 -timeout 120s
```

Kiểm thử seed tạo rồi xóa schema MySQL riêng, không reset database ứng dụng.
