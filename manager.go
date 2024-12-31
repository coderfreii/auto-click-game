package main

import (
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/go-vgo/robotgo"
	"gocv.io/x/gocv"
)

type ManagerConfig struct {
	RefreshTime int    //刷新时间，单位毫秒
	ImgPath     string //样本文件夹
}

type ImgInfo struct {
	path    string //图片路径
	ImgMaxX int
	ImgMaxY int
	Img     image.Image
}

type Manager struct {
	ManagerConfig

	ImgInfos []ImgInfo
}

func NewManager(cfg ManagerConfig) Manager {
	imgInfos := make([]ImgInfo, 0)
	files, err := ioutil.ReadDir(cfg.ImgPath)
	fmt.Println(err)
	for _, imgFile := range files {
		if reader, err := os.Open(filepath.Join(cfg.ImgPath, imgFile.Name())); err == nil {
			defer reader.Close()

			if reader.Name() == filepath.Join(cfg.ImgPath, ".DS_Store") {
				continue
			}

			im, _, err := image.DecodeConfig(reader)
			if err != nil {
				fmt.Println(imgFile.Name(), err)
				panic("图片错误")
			}

			img := readPic(filepath.Join(cfg.ImgPath, imgFile.Name()))

			fmt.Println("已读取", imgFile.Name())
			imgInfos = append(imgInfos, ImgInfo{
				path:    cfg.ImgPath + imgFile.Name(),
				ImgMaxX: im.Width,
				ImgMaxY: im.Height,
				Img:     img,
			})
		} else {
			panic(err)
		}
	}

	return Manager{
		ImgInfos:      imgInfos,
		ManagerConfig: cfg,
	}
}

// 根据img中找到的的temp左上角坐标，和temp的最大长宽，计算出一块可以点击的区域，并随机点击
func randomClick(tempPosX, tempPosY, tempMaxX, tempMaxY int) {
	//用qq截图软件截下来的图，分辨率是真实分辨率二倍，所以除以2以对应真实分辨率
	tempPosX, tempPosY, tempMaxX, tempMaxY = tempPosX/2, tempPosY/2, tempMaxX/2, tempMaxY/2

	//计算按钮的中心点
	centerX, centerY := tempPosX+tempMaxX/2, tempPosY+tempMaxY/2

	//在中心点加或减offset就是随机坐标的上限和下限
	offsetX := tempMaxX / 2
	offsetY := tempMaxY / 2
	_, randomX := RandomNormalInt64(int64(centerX-offsetX), int64(centerX+offsetX), int64(centerX), 10)
	_, randomY := RandomNormalInt64(int64(centerY-offsetY), int64(centerY+offsetY), int64(centerY), 10)

	robotgo.MouseSleep = 10
	robotgo.Move(int(randomX), int(randomY))
	robotgo.Click("left")
}

func (m *Manager) work() {
	// 创建一个窗口用于显示图像
	window := gocv.NewWindow("Screen Capture")
	defer window.Close()

	fmt.Println("")
	color.Cyan("                 m                         \"\"#      \"           #\n  mmm   m   m  mm#mm   mmm           mmm     #    mmm     mmm   #   m\n \"   #  #   #    #    #\" \"#         #\"  \"    #      #    #\"  \"  # m\"\n m\"\"\"#  #   #    #    #   #   \"\"\"   #        #      #    #      #\"#\n \"mm\"#  \"mm\"#    \"mm  \"#m#\"         \"#mm\"    \"mm  mm#mm  \"#mm\"  #  \"m")
	fmt.Println("")

	bold := color.New(color.Bold).Add(color.FgGreen)
	bold.Println("开始运行脚本，请切换到游戏界面")
	for true {
		//捕获当前屏幕
		start := time.Now()
		screenImg, _ := robotgo.CaptureImg()
		// updateImage(window, screenImg)

		cost := time.Since(start)

		fmt.Println("_______________________________")
		fmt.Println("成功捕获并保存当前屏幕，耗时：", cost)

		//逐一匹配样板图片
		for _, img := range m.ImgInfos {
			start := time.Now()
			tempPosX, tempPosY, num := findTempPosWithFeatures(img.Img, screenImg)
			cost := time.Since(start)

			fmt.Print(" 正在匹配：", img.path, " 相似度：", num, " 匹配耗时：", cost)

			if num > 0.7 {
				start := time.Now()
				randomClick(tempPosX, tempPosY, img.ImgMaxX, img.ImgMaxY)
				cost := time.Since(start)
				bold.Println(" 匹配成功, 耗时：", cost)
				break
			}
			fmt.Println(" 匹配不到相似的图片")
		}
		time.Sleep(time.Duration(m.RefreshTime) * time.Millisecond)
	}
}
