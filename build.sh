GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ssh_toolkits_x86
GOOS=linux GOARCH=arm go build -ldflags "-s -w" -o ssh_toolkits_arm
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ssh_toolkits_x86.exe
GOOS=windows GOARCH=arm go build -ldflags "-s -w" -o ssh_toolkits_arm.exe




# 执行命令

nohup ssh_toolkits -port 4400 -username tools -password tools &

