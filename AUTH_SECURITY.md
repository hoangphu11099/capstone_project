# Xác thực đăng nhập

## Luồng request và response

1. Client gửi `POST /login` với JSON `{ "username": "...", "password": "..." }`. Backend kiểm tra mật khẩu bằng bcrypt và trạng thái tài khoản.
2. Response thành công có `token`, `token_type: "Bearer"`, `expires_in: 86400`, `first_login` và thông tin người dùng; không chứa mật khẩu hay hash mật khẩu. Response đăng nhập không được cache.
3. Request tiếp theo gửi `Authorization: Bearer <JWT>`. Middleware yêu cầu JWT HS256 hợp lệ, đúng issuer/audience, có thời hạn và ID phiên. Việc giới hạn thuật toán và yêu cầu thời hạn dùng các tùy chọn xác thực của [golang-jwt/jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5).
4. Mỗi token gắn với một dòng `auth_sessions`. Backend kiểm tra phiên chưa hết hạn, tài khoản còn hoạt động, quyền hiện tại và dấu kiểm tra mật khẩu trước khi cho phép truy cập. Quyền lưu trên client không quyết định quyền API.
5. `POST /logout` xóa phiên tương ứng, nên JWT đã đăng xuất không thể dùng lại. Đổi hoặc đặt lại mật khẩu làm token cũ của các thiết bị mất hiệu lực. Đổi mật khẩu lần đầu trả token mới và client phải thay token cũ.

## Các thay đổi về mật khẩu

- Tài khoản `first_login` chỉ được đổi mật khẩu hoặc đăng xuất; không được truy cập chức năng khác trước khi đổi mật khẩu.
- Ngoài lần đăng nhập đầu, `POST /change-password` yêu cầu `current_password` cùng `new_password`.
- `/forgot-password` công khai chỉ hướng dẫn liên hệ nhà trường để xác minh danh tính. Endpoint này không đổi mật khẩu và không trả mật khẩu tạm. Admin vẫn có thể cấp lại mật khẩu qua chức năng chỉnh sửa tài khoản sinh viên/giảng viên hiện có, sau khi xác minh danh tính.
- Hệ thống hiện dùng access token thời hạn 24 giờ; chưa có refresh token. Khi hết hạn, người dùng đăng nhập lại.

## Lưu trữ và cấu hình

- App native lưu JWT bằng [Expo SecureStore](https://docs.expo.dev/versions/latest/sdk/securestore/). Bản token cũ trong AsyncStorage được chuyển sang SecureStore rồi xóa. Client không gửi JWT sang URL ngoài API đã cấu hình.
- Dashboard web vẫn lưu bearer token trong localStorage/sessionStorage theo lựa chọn ghi nhớ đăng nhập. Cách này không bảo vệ token khỏi JavaScript độc hại nếu có XSS; chưa chuyển sang cookie HttpOnly.
- `JWT_SECRET` lấy từ môi trường sau khi nạp `.env`, tối thiểu 32 ký tự ngẫu nhiên. Không dùng khóa mẫu, không commit giá trị thật. Production bắt buộc cấu hình khóa; development không cấu hình dùng khóa ngẫu nhiên cho mỗi lần chạy.
- Khi đưa lên máy chủ phải cấu hình HTTPS ở API/reverse proxy. JWT ký xác thực token, không mã hóa mật khẩu hoặc dữ liệu truyền qua HTTP.
- Chỉ đếm đăng nhập sai thông tin xác thực (HTTP 401), tách riêng từng tài khoản + IP. Đăng nhập đúng không bị cộng số lần và xóa chuỗi lần sai trước đó. Sau 30 lần sai liên tiếp, cặp tài khoản + IP tạm chờ 5 phút; phản hồi `Retry-After` cho biết số giây còn lại. Các tài khoản khác dùng chung Wi-Fi không bị cộng dồn lỗi của nhau. Lỗi máy chủ và lỗi dữ liệu đầu vào không được tính là sai mật khẩu. Bộ đếm tự hết hạn sau 5 phút không có lần sai mới, áp dụng trên từng tiến trình backend. Nếu chạy nhiều instance, cần giới hạn chung tại reverse proxy hoặc kho dùng chung. Chỉ cấu hình `TRUSTED_PROXIES` bằng IP/CIDR proxy thực tế.
- Khởi động lại backend để AutoMigrate tạo `auth_sessions`. Token đã cấp trước bản cập nhật không đáp ứng kiểm tra phiên mới; người dùng cần đăng nhập lại. Thay khóa JWT cũng yêu cầu đăng nhập lại.

## Kiểm tra tự động

- `go test ./utils ./middleware`: JWT hết hạn, thiếu claim bắt buộc, sai issuer/audience/thuật toán, chữ ký bị sửa và giới hạn đăng nhập.
- `RUN_ACADEMIC_INTEGRATION=1 go test ./controllers -run '^TestAcademicFlowIntegration/JWT_sessions'`: luồng đăng nhập thực tế, đăng xuất, đổi mật khẩu, thay đổi quyền, khóa tài khoản, bắt buộc đổi mật khẩu lần đầu và khôi phục mật khẩu công khai. Kiểm thử tạo và xóa schema MySQL riêng, không dùng schema ứng dụng.
