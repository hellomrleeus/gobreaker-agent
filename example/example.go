package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hellomrleeus/gobreaker/agent"
)

var as *agent.Agents

func init() {
	name := "MOCK_GET"
	config := agent.Config{
		Settings: agent.Settings{
			Name: name,
			ReadyToTrip: func(counts agent.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests > 3 && failureRatio >= 0.2
			},
			Timeout: 10 * time.Second,
		},
		Limit: 0.5,
		Burst: 2,
	}
	as = agent.NewAgents()
	as.M.Store(name, agent.NewAgent(config))
}

func fakeGet() (interface{}, error) {
	time.Sleep(time.Second)
	n := rand.Intn(3)
	if n < 1 {
		return "success", nil
	}
	return nil, errors.New("fail")
}

func main() {
	ag, ok := as.M.Load("MOCK_GET")
	if !ok {
		fmt.Println("??")
	}

	t := ag.(*agent.Agent)
	cb := t.Breaker
	l := t.Limiter
	for i := 0; i < 1000; i++ {
		go func() {
			body, err := cb.Execute(context.TODO(), l, func() (interface{}, error) {
				resp, err := fakeGet()
				if err != nil {
					return nil, err
				}
				return resp, nil
			})
			fmt.Println(body, err)
		}()
	}

	for {
	}
}
