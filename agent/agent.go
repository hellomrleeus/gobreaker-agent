package agent

import (
	"sync"

	gobreaker "github.com/hellomrleeus/gobreaker/breaker"
)

//Agents Agent集合
type Agents struct {
	M sync.Map
}

//NewAgents 初始化
func NewAgents() *Agents {
	return &Agents{}
}

//Settings gobreaker.Settings 别名
type Settings = gobreaker.Settings

//Limit gobreaker.Limit 别名
//type Limit = gobreaker.Limit

//Counts gobreaker.Counts 别名
type Counts = gobreaker.Counts

//Config Agent配置项
//Settings 组合自gobreaker.Settings
//Limit, Burst 组合自gobreaker.Limit
type Config struct {
	Settings Settings
	Limit    float64
	Burst    int
}

//Agent 请求代理
type Agent struct {
	Name    string
	Breaker *gobreaker.CircuitBreaker
	Limiter *gobreaker.Limiter
}

//NewAgent 初始化
func NewAgent(c Config) *Agent {
	return &Agent{
		Name:    c.Settings.Name,
		Breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings(c.Settings)),
		Limiter: gobreaker.NewLimiter(c.Limit, c.Burst),
	}
}
