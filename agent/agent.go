package agent

import (
	"sync"

	"github.com/hellomrleeus/gobreaker-agent/breaker"
)

//Agents Agent集合
type Agents struct {
	M sync.Map
}

//NewAgents 初始化
func NewAgents() *Agents {
	return &Agents{}
}

//Settings breaker.Settings 别名
type Settings = breaker.Settings

//Counts breaker.Counts 别名
type Counts = breaker.Counts

//Config Agent配置项
//Settings 组合自breaker.Settings
//Limit, Burst 组合自breaker.Limit
type Config struct {
	Settings Settings
	Limit    breaker.Limit
	Burst    int
}

//Agent 请求代理
type Agent struct {
	Name    string
	Breaker *breaker.CircuitBreaker
	Limiter *breaker.Limiter
}

//NewAgent 初始化
func NewAgent(c Config) *Agent {
	return &Agent{
		Name:    c.Settings.Name,
		Breaker: breaker.NewCircuitBreaker(breaker.Settings(c.Settings)),
		Limiter: breaker.NewLimiter(c.Limit, c.Burst),
	}
}
