package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"project/config"
	"project/global"
	"project/log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 硬编码
var (
	Accurate_District = [10][2]string{
		{"北京", "北京市"},
		{"上海", "上海市"},
		{"广东省", "深圳市"},
		{"广东省", "广州市"},
		{"浙江省", "杭州市"},
		{"江苏省", "南京市"},
		{"重庆市", "重庆市"},
		{"湖北省", "武汉市"},
		{"陕西省", "西安市"},
		{"辽宁省", "沈阳市"},
	}
	district_country_choice = false
)

// 返回给前端的数据-这里是10条城市的简化数据
type CitySummary struct {
	Province    string `json:"province"`
	City        string `json:"city"`
	County      string `json:"county,omitempty"`
	Degree      string `json:"degree,omitempty"`
	Weather     string `json:"weather,omitempty"`
	WeatherUrl  string `json:"weather_url,omitempty"` //  天气的图标url
	AQI         int    `json:"aqi,omitempty"`
	AQIName     string `json:"aqi_name,omitempty"`
	TomorrowMin string `json:"tomorrow_min,omitempty"`
	TomorrowMax string `json:"tomorrow_max,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Error       string `json:"error,omitempty"` // 错误信息
}

// 这里只需取一个用的数据
// GetUser_info - 返回统一结构，方便前端只调用一个接口拿到 name + loc + weather
func GetUser_Info(c *gin.Context) {
	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	base := c.Request.Context()
	ctx, cancel := context.WithTimeout(base, global.Timeout)
	defer cancel()

	loc, err := getLocalLocation()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user's location", "detail": err.Error()})
		return
	}
	province, city := loc[0], loc[1]
	// 这个返回很关键
	userWeather, err := fetchWithCache_Weather(ctx, province, city, "", false)
	if err != nil {
		// 把错误信息一并返回，前端可以展示
		c.JSON(http.StatusOK, gin.H{
			"name":         uname,
			"user_loc":     gin.H{"province": province, "city": city},
			"user_weather": nil,
			"error":        err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":         uname, // <-- 关键字段，前端以前没有拿到的就是这个
		"user_loc":     gin.H{"province": province, "city": city},
		"user_weather": userWeather,
	})
}

// 对于请求的循环最好用并行循环-取10个地区的数据
func GetWeatherData_top10(c *gin.Context) {
	uid := c.GetUint("user_id")
	uname := c.GetString("username")
	if uid == 0 || uname == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	base := c.Request.Context()
	ctx, cancel := context.WithTimeout(base, global.Timeout)
	defer cancel()

	type district struct{ Prov, City, County string }
	dsts := make([]district, 0, len(Accurate_District))
	for i := 0; i < len(Accurate_District); i++ {
		// 防御：确保每条 Accurate_District 至少含两项
		if len(Accurate_District[i]) < 2 {
			continue
		}
		prov := Accurate_District[i][0]
		city := Accurate_District[i][1]
		county := ""
		dsts = append(dsts, district{Prov: prov, City: city, County: county})
	}

	out := make([]CitySummary, len(dsts))
	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i := range dsts {
		idx := i
		dd := dsts[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			cs, err := fetchWithCache_Weather(ctx, dd.Prov, dd.City, dd.County, false)
			if err != nil {
				log.L().Warn("Failed to fetch city's data",
					zap.String("prov", dd.Prov), zap.String("city", dd.City), zap.Error(err))
				// 返回有意义的占位结构，便于前端展示错误信息
				cs = CitySummary{
					Province: dd.Prov,
					City:     dd.City,
					County:   dd.County,
					Error:    err.Error(),
				}
			} else {
				// 防御：若 fetch 返回但缺省 city/province，填上
				if cs.City == "" {
					cs.City = dd.City
				}
				if cs.Province == "" {
					cs.Province = dd.Prov
				}
			}
			out[idx] = cs
		}()
	}
	wg.Wait() // 并行停止
	// 直接返回数组（客户端按数组解析）
	c.JSON(http.StatusOK, out)
}

// 生成对应的城市的redis缓存-key

func fetchWithCache_Weather(ctx context.Context, prov, city, county string, force bool) (CitySummary, error) {
	key := cacheKey(prov, city, county)

	if !force {
		if b, err := global.RedisDB.Get(key).Bytes(); err == nil {
			var cached CitySummary
			if json.Unmarshal(b, &cached) == nil {
				return cached, nil
			}
		}
	}

	v, err, _ := global.FetchGroup.Do(key, func() (interface{}, error) { // 并发保证用户多个请求这里只先执行一次
		reqCtx, cancel := context.WithTimeout(ctx, global.FetchTimeout)
		defer cancel()

		wd, e := getWeatherDataParsed(reqCtx, prov, city, county)
		if e != nil {
			// 降级：尝试旧缓存
			if b, ge := global.RedisDB.Get(key).Bytes(); ge == nil {
				var stale CitySummary
				if json.Unmarshal(b, &stale) == nil { //用到旧缓存不报错-在force为true前提下
					return stale, nil
				}
			}
			return CitySummary{Province: prov, City: city, County: county, Error: e.Error()}, e // 直接报错
		}

		cs := summaryFromWeatherData(prov, city, county, wd)
		if jb, e2 := json.Marshal(cs); e2 == nil {
			_ = global.RedisDB.Set(key, jb, global.CacheTTL).Err()
		}
		return cs, nil
	})

	// 上述Do的总分情况
	if err != nil {
		if v != nil {
			if cs, ok := v.(CitySummary); ok {
				return cs, nil
			}
		}
		return CitySummary{Province: prov, City: city, County: county, Error: err.Error()}, err
	}
	return v.(CitySummary), nil
}

// ======获取位置信息=====
// LocationInfo 结构体用于存储从API获取的位置信息
type LocationInfo struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Province string `json:"province"`
	City     string `json:"city"`
	District string `json:"district"`
}

func getLocationByIP() (LocationInfo, error) {
	ipUrl := fmt.Sprintf("https://restapi.amap.com/v3/ip?key=%s", config.LocalAPIKey) //提前创造好的
	client := &http.Client{Timeout: 10 * time.Second}  // 创建一个客户端的结构体实例并赋值

	// IP定位获取城市信息
	resp, err := client.Get(ipUrl) //依据Url获取定位
	if err != nil {
		return LocationInfo{}, fmt.Errorf("IP定位请求失败: %v", err)
	}
	defer resp.Body.Close()          // 关闭响应体，释放网络链接
	body, _ := io.ReadAll(resp.Body) // 读取整个响应体到内存
	var ipResult struct {
		Status   string      `json:"status"`
		Info     string      `json:"info"`
		Province string      `json:"province"`
		City     string      `json:"city"`
		District interface{} `json:"district"`
		Adcode   string      `json:"adcode"` // 可以保留adcode字段，有时它可能包含有用的区域编码信息
	}
	if err := json.Unmarshal(body, &ipResult); err != nil {
		return LocationInfo{}, fmt.Errorf("解析IP定位响应失败: %v", err)
	}
	if ipResult.Status != "1" {
		return LocationInfo{}, fmt.Errorf("IP定位失败: %s", ipResult.Info)
	}
	var district string
	switch ipResult.District.(type) {
	case nil:
		district = ""
	case string:
		district = ipResult.District.(string)
	}

	return LocationInfo{
		Status:   ipResult.Status,
		Province: ipResult.Province,
		City:     ipResult.City,
		District: district, // 一般来说不返回区县信息，因为IP定位无法准确获取
	}, nil
}

func getLocalLocation() ([]string, error) { //返回三个字符数组
	// 获取IP位置信息
	location, err := getLocationByIP()
	if err != nil {
		return []string{}, err //报错
	}
	// 查找匹配的区域
	var loc []string
	loc = append(loc, location.Province) // 返回新的切片
	loc = append(loc, location.City)
	if location.District != "" {
		loc = append(loc, location.District)
	}
	return loc, nil
}

// ------------------------------------------------------------------
// 天气数据
// 定义天气数据结构体
// WeatherData 天气数据结构体，包含当前天气、空气质量和24小时预报信息
type WeatherData struct {
	// Status API请求状态码，通常1表示成功，其他值表示失败
	Status int `json:"status"`
	// Message API返回的消息，可能是错误信息或状态描述
	Message string `json:"message"`
	// Data 包含所有天气数据的嵌套结构
	Data struct {
		// Air 空气质量数据
		Air struct {
			// AQI 空气质量指数，数值越大表示空气污染越严重
			AQI int `json:"aqi"`
			// AQILevel 空气质量等级，通常1-6级对应优到严重污染
			AQILevel int `json:"aqi_level"`
			// AQIName 空气质量等级名称，如"优"、"良"、"轻度污染"等
			AQIName string `json:"aqi_name"`
			// AQIUrl 空气质量图标链接，用于显示对应等级的图标
			AQIUrl string `json:"aqi_url"`
			// CO 一氧化碳浓度，单位通常是mg/m³
			CO string `json:"co"`
			// NO2 二氧化氮浓度，单位通常是μg/m³
			NO2 string `json:"no2"`
			// O3 臭氧浓度，单位通常是μg/m³
			O3 string `json:"o3"`
			// PM10 PM10颗粒物浓度，单位通常是μg/m³
			PM10 string `json:"pm10"`
			// PM25 PM2.5颗粒物浓度，单位通常是μg/m³
			PM25 string `json:"pm2_5"`
			// SO2 二氧化硫浓度，单位通常是μg/m³
			SO2 string `json:"so2"`
			// UpdateTime 空气质量数据更新时间
			UpdateTime string `json:"update_time"`
			// Rank 该城市在全国空气质量排名
			Rank int `json:"rank"`
			// Total 全国参与排名的城市总数
			Total int `json:"total"`
		} `json:"air"`

		// Observe 当前天气观测数据
		Observe struct {
			// Degree 当前温度，单位通常是摄氏度
			Degree string `json:"degree"`
			// Humidity 当前相对湿度，单位是百分比
			Humidity string `json:"humidity"`
			// Precipitation 当前降水量，单位通常是毫米
			Precipitation string `json:"precipitation"`
			// Pressure 当前大气压，单位通常是百帕(hPa)
			Pressure string `json:"pressure"`
			// UpdateTime 天气数据更新时间
			UpdateTime string `json:"update_time"`
			// Weather 天气状况描述，如"晴"、"多云"、"小雨"等
			Weather string `json:"weather"`
			// WeatherCode 天气状况代码，便于程序判断天气类型
			WeatherCode string `json:"weather_code"`
			// WeatherShort 天气状况的简短描述
			WeatherShort string `json:"weather_short"`
			// WindDirection 风向，可能用角度或方位表示
			WindDirection string `json:"wind_direction"`
			// WindPower 风力等级或风速
			WindPower string `json:"wind_power"`
			// WindDirectionName 风向名称，如"东北风"、"南风"等
			WindDirectionName string `json:"wind_direction_name"`
			// WeatherBgPag 天气背景图片链接，用于UI背景
			WeatherBgPag string `json:"weather_bg_pag"`
			// WeatherPag 天气动画图片链接，用于动态效果
			WeatherPag string `json:"weather_pag"`
			// WeatherUrl 天气图标链接，用于显示当前天气图标
			WeatherUrl string `json:"weather_url"`
			// WeatherColor 天气相关的颜色数组，可用于UI设计
			WeatherColor []string `json:"weather_color"`
			// WeatherFirst 天气首屏图片链接，用于应用启动或主界面
			WeatherFirst string `json:"weather_first"`
		} `json:"observe"`

		// Forecast24h 24小时天气预报，键是时间字符串，值是预报数据
		Forecast24h map[string]struct {
			// DayWeather 白天天气状况描述
			DayWeather string `json:"day_weather"`
			// DayWeatherCode 白天天气状况代码
			DayWeatherCode string `json:"day_weather_code"`
			// DayWeatherShort 白天天气状况简短描述
			DayWeatherShort string `json:"day_weather_short"`
			// DayWindDirection 白天风向
			DayWindDirection string `json:"day_wind_direction"`
			// DayWindDirectionCode 白天风向代码
			DayWindDirectionCode string `json:"day_wind_direction_code"`
			// DayWindPower 白天风力
			DayWindPower string `json:"day_wind_power"`
			// DayWindPowerCode 白天风力代码
			DayWindPowerCode string `json:"day_wind_power_code"`
			// MinDegree 最低温度
			MinDegree string `json:"min_degree"`
			// MaxDegree 最高温度
			MaxDegree string `json:"max_degree"`
			// NightWeather 夜间天气状况描述
			NightWeather string `json:"night_weather"`
			// NightWeatherCode 夜间天气状况代码
			NightWeatherCode string `json:"night_weather_code"`
			// NightWeatherShort 夜间天气状况简短描述
			NightWeatherShort string `json:"night_weather_short"`
			// NightWindDirection 夜间风向
			NightWindDirection string `json:"night_wind_direction"`
			// NightWindDirectionCode 夜间风向代码
			NightWindDirectionCode string `json:"night_wind_direction_code"`
			// NightWindPower 夜间风力
			NightWindPower string `json:"night_wind_power"`
			// NightWindPowerCode 夜间风力代码
			NightWindPowerCode string `json:"night_wind_power_code"`
			// Time 预报时间点
			Time string `json:"time"`
			// AQI 该时段的空气质量指数
			AQI int `json:"aqi"`
			// AQILevel 该时段的空气质量等级
			AQILevel int `json:"aqi_level"`
			// AQIName 该时段的空气质量等级名称
			AQIName string `json:"aqi_name"`
			// AQIUrl 该时段的空气质量图标链接
			AQIUrl string `json:"aqi_url"`
			// DayWeatherUrl 白天天气图标链接
			DayWeatherUrl string `json:"day_weather_url"`
			// NightWeatherUrl 夜间天气图标链接
			NightWeatherUrl string `json:"night_weather_url"`
		} `json:"forecast_24h"`
	} `json:"data"`
}

// var url := "https://wis.qq.com/weather/common?source=pc&weather_type=observe|forecast_24h|air&province=" +   province + "&city=" + city

func getWeatherData(ctx context.Context, province, city, county string) ([]byte, error) {
	// 构建URL
	// 简化方案2：使用map和条件判断
	url := "https://wis.qq.com/weather/common?source=pc&weather_type=observe|forecast_24h|air&province=" + province + "&city=" + city
	if district_country_choice {
		url += "&county=" + county
	}

	// 创建带超时的HTTP客户端用户-借它来发送请求
	client := &http.Client{
		Timeout: global.Timeout,
	}
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.L().Error("The creation request failed !", zap.Error(err))
		return nil, err
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		log.L().Error("The Request failed !", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		log.L().Error(fmt.Sprintf("Abnormal response status code :%d!", resp.StatusCode), zap.Error(err))
		return nil, fmt.Errorf("HTTP response code: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.L().Error("Failed to read response body !", zap.Error(err))
		return nil, err
	}

	return body, nil //这里的body是原始的JSON数据
}

// 获得解析后的天气数据
func getWeatherDataParsed(ctx context.Context, province, city, county string) (*WeatherData, error) {
	// 获取原始数据
	body, err := getWeatherData(ctx, province, city, county)
	if err != nil {
		return nil, err
	}
	// 解析传来的JSON数据
	var weatherData WeatherData
	err = json.Unmarshal(body, &weatherData)
	if err != nil {
		log.L().Error("Failed to parse weatherData JSON!", zap.Error(err))
		return nil, err
	}
	return &weatherData, nil
}

// ===辅助函数===
func cacheKey(prov, city, county string) string {
	if county == "" {
		return fmt.Sprintf("weather:%s:%s", prov, city)
	}
	return fmt.Sprintf("weather:%s:%s:%s", prov, city, county)
}
func summaryFromWeatherData(prov, city, county string, wd *WeatherData) CitySummary {
	s := CitySummary{Province: prov, City: city, County: county}
	if wd == nil { // 没有对应的数据
		s.Error = "no data"
		return s
	}
	s.Degree = wd.Data.Observe.Degree
	s.Weather = wd.Data.Observe.Weather
	s.WeatherUrl = wd.Data.Observe.WeatherUrl // 天气的图标url
	s.UpdatedAt = wd.Data.Observe.UpdateTime
	s.AQI = wd.Data.Air.AQI
	s.AQIName = wd.Data.Air.AQIName
	if f, ok := wd.Data.Forecast24h["1"]; ok { // 如果这个是对的则存入当前的数据
		s.TomorrowMin = f.MinDegree
		s.TomorrowMax = f.MaxDegree
	}
	return s
}

// 使用示例-测试
func displayWeatherWithImages(ctx context.Context, province, city, county string) {
	weather, err := getWeatherDataParsed(ctx, province, city, county)
	if err != nil {
		log.L().Error("Failed to parse weatherData!", zap.Error(err))
	}

	// 当前天气信息
	fmt.Printf("当前温度: %s°C\n", weather.Data.Observe.Degree)
	fmt.Printf("天气状况: %s\n", weather.Data.Observe.Weather)
	fmt.Printf("湿度: %s%%\n", weather.Data.Observe.Humidity)
	fmt.Printf("风向: %s\n", weather.Data.Observe.WindDirectionName)
	fmt.Printf("风力: %s级\n", weather.Data.Observe.WindPower)

	// 当前天气图片链接
	fmt.Printf("天气图标: %s\n", weather.Data.Observe.WeatherUrl)
	fmt.Printf("天气背景: %s\n", weather.Data.Observe.WeatherBgPag)
	fmt.Printf("天气动画: %s\n", weather.Data.Observe.WeatherPag)

	// 空气质量
	fmt.Printf("空气质量: %s (AQI: %d)\n", weather.Data.Air.AQIName, weather.Data.Air.AQI)
	fmt.Printf("空气质量图标: %s\n", weather.Data.Air.AQIUrl)

	// 未来天气预报
	if forecast, ok := weather.Data.Forecast24h["1"]; ok {
		fmt.Printf("明天天气: %s, 温度: %s°C - %s°C\n",
			forecast.DayWeather, forecast.MinDegree, forecast.MaxDegree)
		fmt.Printf("明天白天天气图标: %s\n", forecast.DayWeatherUrl)
		fmt.Printf("明天夜晚天气图标: %s\n", forecast.NightWeatherUrl)
	}
}
