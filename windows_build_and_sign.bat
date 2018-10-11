set GOOS=windows
set GOARCH=386
go build -o certcheck_windows_386.exe
go build -o certcheck_windows_386_sign.exe
set GOARCH=amd64
go build -o certcheck_windows_amd64.exe
go build -o certcheck_windows_amd64_sign.exe
"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe" sign /v /a /tr http://timestamp.globalsign.com/?signature=sha2 /td sha256 /fd sha256 certcheck_windows_amd64_sign.exe
"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe" sign /v /a /tr http://timestamp.globalsign.com/?signature=sha2 /td sha256 /fd sha256 certcheck_windows_386_sign.exe
