package main

import (
	"fmt"
	"image"
	"image/draw"

	"gocv.io/x/gocv"
)

// func findTempPos(temp, img image.Image) (int, int, float32) {
// 	bestX, bestY := -1, -1
// 	bestNum := float32(0)

// 	tempRGBA := jpg2RGBA(temp)
// 	imgRGBA := jpg2RGBA(img)

// 	// 生成多尺度图像
// 	scales := []float64{0.5, 0.75, 1.0, 1.25, 1.5} // 不同缩放比例
// 	for _, scale := range scales {
// 		scaledTemp := resizeImage(tempRGBA, scale)
// 		_, num, _, pos := gcv.FindImg(scaledTemp, imgRGBA)
// 		if num > bestNum { // 找到更优匹配结果
// 			bestNum = num
// 			bestX = pos.X
// 			bestY = pos.Y
// 		}
// 	}

// 	return bestX, bestY, bestNum
// }

// // 进行图像识别，在img中找temp，并返回在img中找到的的temp左上角坐标
// func findTempPos(temp, img image.Image) (int, int, float32) {
// 	//把image.Image统一转换成image.RGBA，不然会断言失败
// 	_, num, _, pos := gcv.FindImg(jpg2RGBA(temp), jpg2RGBA(img))
// 	return pos.X, pos.Y, num
// }

// 将 image.Image 转换为 gocv.Mat
func ImageToMatRGB(img image.Image) (gocv.Mat, error) {
	// 处理图像转换为 Mat 的逻辑
	mat, err := gocv.ImageToMatRGBA(img)
	if err != nil {
		return gocv.NewMat(), fmt.Errorf("failed to convert image to Mat: %v", err)
	}
	return mat, nil
}

func findTempPosWithFeatures(temp, img image.Image) (int, int, float32) {
	// 转换图像为 Mat
	tempMat, err := ImageToMatRGB(temp)
	if err != nil {
		fmt.Println("Error converting temp image:", err)
		return -1, -1, 0
	}
	imgMat, err := ImageToMatRGB(img)
	if err != nil {
		fmt.Println("Error converting img image:", err)
		return -1, -1, 0
	}

	// 将图像转换为灰度图
	tempMatGray := gocv.NewMat()
	imgMatGray := gocv.NewMat()
	defer tempMatGray.Close()
	defer imgMatGray.Close()

	gocv.CvtColor(tempMat, &tempMatGray, gocv.ColorBGRToGray)
	gocv.CvtColor(imgMat, &imgMatGray, gocv.ColorBGRToGray)

	// 提取 ORB 特征点
	orb := gocv.NewORB()
	defer orb.Close()

	// 检测并计算特征点描述子
	kpTemp, descTemp := orb.DetectAndCompute(tempMatGray, gocv.NewMat())
	kpImg, descImg := orb.DetectAndCompute(imgMatGray, gocv.NewMat())

	// 特征点匹配：使用暴力匹配器（BFMatcher）
	matcher := gocv.NewBFMatcherWithParams(gocv.NormL2, false) // 使用 NormL2 范数，crossCheck 设为 false
	defer matcher.Close()
	matches := matcher.KnnMatch(descTemp, descImg, 2) // KNN 匹配，返回 2 个最好的匹配

	// 筛选好的匹配点（比率检验：低于 0.75 的比率被认为是好的匹配）
	goodMatches := make([]gocv.DMatch, 0)
	for _, m := range matches {
		if len(m) == 2 && m[0].Distance < 0.*m[1].Distance {
			goodMatches = append(goodMatches, m[0])
		}
	}

	// 如果有足够的匹配点，计算单应性矩阵
	if len(goodMatches) > 4 {
		pointsTemp := make([]gocv.Point2f, len(goodMatches))
		pointsImg := make([]gocv.Point2f, len(goodMatches))

		// 获取匹配点的坐标
		for i, match := range goodMatches {
			pointsTemp[i] = gocv.Point2f{X: float32(kpTemp[match.QueryIdx].X), Y: float32(kpTemp[match.QueryIdx].Y)}
			pointsImg[i] = gocv.Point2f{X: float32(kpImg[match.TrainIdx].X), Y: float32(kpImg[match.TrainIdx].Y)}
		}

		// 创建 Mat 来存储点
		srcPoints := gocv.NewMatWithSize(len(pointsTemp), 1, gocv.MatTypeCV32FC2)
		dstPoints := gocv.NewMatWithSize(len(pointsImg), 1, gocv.MatTypeCV32FC2)
		defer srcPoints.Close()
		defer dstPoints.Close()

		// 使用 Set方法将点添加到 Mat
		for i, pt := range pointsTemp {
			srcPoints.SetFloatAt(i, 0, pt.X)
			srcPoints.SetFloatAt(i, 1, pt.Y)
		}
		for i, pt := range pointsImg {
			dstPoints.SetFloatAt(i, 0, pt.X)
			dstPoints.SetFloatAt(i, 1, pt.Y)
		}

		// 设置 RANSAC 参数：调整重投影阈值、最大迭代次数和置信度
		ransacReprojThreshold := 3.0 // 重投影误差的阈值，越小越严格
		maxIters := 2000             // 最大迭代次数，防止算法过早停止
		confidence := 0.995          // 置信度，控制最终计算的精度

		mask := gocv.NewMat()
		defer mask.Close()

		// 使用 RANSAC 计算单应性矩阵
		homography := gocv.FindHomography(srcPoints, &dstPoints, gocv.HomographyMethodRANSAC, ransacReprojThreshold, &mask, maxIters, confidence)

		// 检查 homography 矩阵有效性
		if homography.Empty() {
			fmt.Println("Invalid homography matrix")
			return -1, -1, 0
		}

		// 创建一个新的 Mat 来存储转换后的点，每个点包含 2 个坐标（X 和 Y）
		pts := gocv.NewMatWithSize(len(pointsTemp), 1, gocv.MatTypeCV32FC2) //
		//defer pts.Close()

		fmt.Println("srcPoints Matrix dimensions:", srcPoints.Rows(), "x", srcPoints.Cols()) // 打印行列数
		fmt.Println("srcPoints Matrix type:", srcPoints.Type())                              // 打印数据类型
		fmt.Println("srcPoints Matrix size:", srcPoints.Size())

		fmt.Println("pts Matrix dimensions:", pts.Rows(), "x", pts.Cols()) // 打印行列数
		fmt.Println("pts Matrix type:", pts.Type())                        // 打印数据类型
		fmt.Println("pts Matrix size:", pts.Size())

		fmt.Println("homography Matrix dimensions:", homography.Rows(), "x", homography.Cols()) // 打印行列数
		fmt.Println("homography Matrix type:", homography.Type())                               // 打印数据类型
		fmt.Println("homography Matrix size:", homography.Size())

		// 确保 homography 与 srcPoints 和 pts 的数据类型一致
		homographyConverted := gocv.NewMat()
		defer homographyConverted.Close()

		// 将 homography 转换为 CV32F 类型
		//homography.ConvertTo(&homographyConverted, gocv.MatTypeCV32F)
		homography.ConvertTo(&homographyConverted, gocv.MatTypeCV32FC2)

		fmt.Println("homographyConverted Matrix dimensions:", homographyConverted.Rows(), "x", homographyConverted.Cols()) // 打印行列数
		fmt.Println("homographyConverted Matrix type:", homographyConverted.Type())                                        // 打印数据类型
		fmt.Println("homographyConverted Matrix size:", homographyConverted.Size())

		// 计算转换后的坐标
		gocv.PerspectiveTransform(srcPoints, &pts, homographyConverted)

		// 提取转换后的坐标
		topLeft := pts.GetVecfAt(0, 0) // 获取第一个转换后的坐标
		return int(topLeft[0]), int(topLeft[1]), float32(len(goodMatches))
	}

	// 如果匹配点不足，返回负值，表示未找到匹配
	return -1, -1, 0
}

// updateImage 方法，每次调用时刷新传入的图像
func updateImage(window *gocv.Window, img image.Image) {
	// 显示传入的图像
	res, _ := convertToMat(img)
	window.IMShow(res)

	// 等待按键，如果按下 'Esc' 键退出
	if gocv.WaitKey(1) == 27 { // 27 是 'Esc' 键的 ASCII 码
		return
	}
}

// 将 robotgo 捕获的图像转换为 gocv 的 Mat
func convertToMat(img image.Image) (gocv.Mat, error) {
	// 获取图像的宽度和高度
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// 创建一个空的 Mat 用于存储图像数据
	mat := gocv.NewMatWithSize(height, width, gocv.MatTypeCV8UC3)
	if mat.Empty() {
		return gocv.Mat{}, fmt.Errorf("无法创建 Mat 对象")
	}
	// defer mat.Close()

	// 将图像转换为 RGBA 格式，并逐像素填充到 Mat 中
	rgbaImg := image.NewRGBA(img.Bounds())
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, image.Point{}, draw.Over)

	// 将图像数据写入到 gocv 的 Mat 中
	for y := 0; y < rgbaImg.Bounds().Dy(); y++ {
		for x := 0; x < rgbaImg.Bounds().Dx(); x++ {
			r, g, b, _ := rgbaImg.At(x, y).RGBA()
			mat.SetUCharAt(y, x*3, uint8(r>>8))   // Red
			mat.SetUCharAt(y, x*3+1, uint8(g>>8)) // Green
			mat.SetUCharAt(y, x*3+2, uint8(b>>8)) // Blue
		}
	}

	return mat, nil
}
