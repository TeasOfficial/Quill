package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quill/internal/config"
	"quill/internal/docserver"
	"quill/internal/logc"
	"quill/internal/module"
	"quill/internal/onebot"
	"quill/internal/plugin"
	"quill/internal/storage"
)

func main() {
	cfg := config.Load()

	fmt.Println()
	fmt.Println(logc.Cyan(`  ___  _   _ ___ _    _    `))
	fmt.Println(logc.Cyan(` / _ \| | | |_ _| |  | |   `))
	fmt.Println(logc.Cyan(`| | | | | | || || |  | |   `))
	fmt.Println(logc.Cyan(`| |_| | |_| || || |__| |___`))
	fmt.Println(logc.Cyan(` \__\_\\___/|___|____|_____/`))
	fmt.Println(logc.Dim(fmt.Sprintf("           v0.1.0-dev  build %s", time.Now().Format("2006-01-02"))))
	fmt.Println()
	logc.Fmt("\u2660", logc.Cyan, "Quill \u542f\u52a8\u4e2d...")
	logc.Fmt("→", logc.Gray, "HTTP: %s  WS: %s  mode: %s", cfg.BaseURL, cfg.WSURL, cfg.WSMode)

	client := onebot.NewClient(cfg.BaseURL, cfg.Token)

	db, err := storage.New("data")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", logc.Red("错误: 创建数据目录失败: "+err.Error()))
		os.Exit(1)
	}

	modHost := module.NewHost()
	defer modHost.Close()

	mgr := plugin.NewManager(client, db, cfg.FileUnsafe)
	defer mgr.Close()

	module.RegisterLuaAPI(mgr.L, modHost)

	if err := mgr.LoadPlugins("plugins"); err != nil {
		logc.Fmt("!", logc.Yellow, "插件加载警告: %v", err)
	}

	builder := module.NewBuilder(modHost, "modules")
	builder.BuildAll()
	if err := builder.Start(); err != nil {
		logc.Fmt("!", logc.Yellow, "模块监听启动失败: %v", err)
	}
	defer builder.Close()

	receiver := onebot.NewReceiver()
	receiver.OnEvent = func(e *onebot.Event) {
		if e.PostType == "meta_event" {
			return
		}
		switch {
		case e.MessageType == "group":
			logc.Fmt("\u25C9", logc.Green, "\u7FA4\u6D88\u606F  [%s] %s", e.Sender.Nickname, trunc(e.RawMessage, 60))
		case e.MessageType == "private":
			logc.Fmt("\u25CE", logc.Blue, "\u79C1\u804A  [%s] %s", e.Sender.Nickname, trunc(e.RawMessage, 60))
		case e.PostType == "notice":
			logNotice(e)
		case e.PostType == "request":
			logc.Fmt("\u25CB", logc.Yellow, "\u8BF7\u6C42  %s: %s", e.RequestType, trunc(e.Comment, 40))
		default:
			logc.Fmt("\u25CB", logc.Gray, "%s", e.PostType)
		}
		mgr.Dispatch(e)
	}
	receiver.OnConnect = func() {
		logc.Fmt("✓", logc.Green, "已连接 OneBot")
	}

	go func() {
		if err := docserver.Serve(":3081"); err != nil {
			logc.Fmt("!", logc.Yellow, "文档服务: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	switch cfg.WSMode {
	case "reverse":
		go func() {
			if err := receiver.Serve(cfg.WSListen, cfg.WSPath, cfg.Token); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", logc.Red("错误: "+err.Error()))
				os.Exit(1)
			}
		}()
	case "forward":
		go func() {
			for {
				if err := receiver.Connect(cfg.WSURL, cfg.Token, cfg.WSPath); err != nil {
					logc.Fmt("↻", logc.Yellow, "WS 连接失败，5秒后重试...")
					time.Sleep(5 * time.Second)
					continue
				}
				receiver.Listen()
				logc.Fmt("↻", logc.Yellow, "WS 断开，5秒后重连...")
				time.Sleep(5 * time.Second)
			}
		}()
	default:
		fmt.Fprintf(os.Stderr, "%s\n", logc.Red("错误: 未知 WS 模式 "+cfg.WSMode))
		os.Exit(1)
	}

	logc.Fmt("☰", logc.Cyan, "运行中，Ctrl+C 退出")
	<-quit
	fmt.Println()
	logc.Fmt("✕", logc.Gray, "正在关闭...")
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

func logNotice(e *onebot.Event) {
	switch e.NoticeType {
	case "group_increase":
		logc.Fmt("\u2192", logc.Green, "\u5165\u7FA4  [%s] \u52A0\u5165\u4E86\u7FA4", e.Sender.Nickname)
	case "group_decrease":
		logc.Fmt("\u2190", logc.Yellow, "\u9000\u7FA4  [%s] \u79BB\u5F00\u4E86\u7FA4", e.Sender.Nickname)
	case "group_ban":
		if e.SubType == "ban" {
			logc.Fmt("\u26D4", logc.Red, "\u7981\u8A00  [%d] \u88AB\u7981\u8A00", e.UserID)
		} else {
			logc.Fmt("\u2713", logc.Green, "\u89E3\u7981  [%d] \u88AB\u89E3\u9664\u7981\u8A00", e.UserID)
		}
	case "group_admin":
		if e.SubType == "set" {
			logc.Fmt("\u2606", logc.Cyan, "\u7FA1\u7BA1\u54E1  [%d] \u6210\u4E3A\u7BA1\u7406\u5458", e.UserID)
		} else {
			logc.Fmt("\u2606", logc.Gray, "\u7FA1\u7BA1\u54E1  [%d] \u53D6\u6D88\u7BA1\u7406\u5458", e.UserID)
		}
	case "group_recall":
		logc.Fmt("\u21B6", logc.Gray, "\u64A4\u56DE  [%d] \u64A4\u56DE\u4E86\u4E00\u6761\u6D88\u606F", e.UserID)
	case "friend_add":
		logc.Fmt("\u2192", logc.Green, "\u597D\u53CB  [%s] \u6DFB\u52A0\u4E86\u597D\u53CB", e.Sender.Nickname)
	case "group_upload":
		if e.File != nil {
			logc.Fmt("\u21E7", logc.Cyan, "\u6587\u4EF6  [%s] %s", e.Sender.Nickname, trunc(e.File.Name, 30))
		} else {
			logc.Fmt("\u21E7", logc.Cyan, "\u6587\u4EF6  [%s] \u4E0A\u4F20\u4E86\u6587\u4EF6", e.Sender.Nickname)
		}
	case "notify":
		switch e.SubType {
		case "poke":
			logc.Fmt("\u261B", logc.Magenta, "\u6233\u4E00\u6233  [%d] \u226B [%d]", e.UserID, e.TargetID)
		case "lucky_king":
			logc.Fmt("\u2654", logc.Yellow, "\u8FD0\u6C14\u738B  [%d] \u6210\u4E3A\u8FD0\u6C14\u738B", e.UserID)
		default:
			logc.Fmt("\u25E6", logc.Gray, "notify.%s [%d]", e.SubType, e.UserID)
		}
	case "essence":
		logc.Fmt("\u2605", logc.Yellow, "\u7CBE\u534E  [%d] \u88AB\u8BBE\u4E3A\u7CBE\u534E\u6D88\u606F", e.MessageID)
	default:
		logc.Fmt("\u25CB", logc.Gray, "notice.%-14s [%s]", e.NoticeType, e.Sender.Nickname)
	}
}
