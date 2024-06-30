package ik

import "sync"

const (
	OnChangeStrategyPre              = "pre"
	OnChangeStrategyPost             = "post"
	OnChangeStrategyConcurrent       = "concurrent"
	OnChangeStrategyConcurrentNoWait = "concurrent-no-wait"
)

type sortedOnChangeCallbacks struct {
	stratPre              []OnChange
	stratConcurrent       []OnChange
	stratPost             []OnChange
	stratConcurrentNoWait []OnChange
	exists                bool
}

func sortOnChangeCallbacks(onChanges []OnChange) sortedOnChangeCallbacks {
	stratPre := []OnChange{}
	stratConcurrent := []OnChange{}
	stratPost := []OnChange{}
	stratConcurrentNoWait := []OnChange{}
	exists := false
	if len(onChanges) == 0 {
		return sortedOnChangeCallbacks{}
	} else {
		exists = true
	}
	for _, o := range onChanges {
		switch o.Strategy {
		case OnChangeStrategyPre, "":
			stratPre = append(stratPre, o)
		case OnChangeStrategyConcurrent:
			stratConcurrent = append(stratConcurrent, o)
		case OnChangeStrategyPost:
			stratPost = append(stratPost, o)
		case OnChangeStrategyConcurrentNoWait:
			stratConcurrentNoWait = append(stratConcurrentNoWait, o)
		}
	}
	return sortedOnChangeCallbacks{
		stratPre:              stratPre,
		stratConcurrent:       stratConcurrent,
		stratPost:             stratPost,
		stratConcurrentNoWait: stratConcurrentNoWait,
		exists:                exists,
	}
}

func (c *Config) runConcurrentOnChangeCallbacks(onChanges *[]OnChange, evtName string, shouldWait bool) {
	if len(*onChanges) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(*onChanges))
		for _, o := range *onChanges {
			if c.getIsIgnored(evtName, &o.ExcludedPatterns) {
				wg.Done()
				continue
			}
			go func(o OnChange) {
				defer wg.Done()
				err := o.Func(evtName)
				if err != nil {
					c.Logger.Errorf("error running extension callback: %v", err)
				}
			}(o)
		}
		if shouldWait {
			wg.Wait()
		}
	}
}

func (c *Config) simpleRunOnChangeCallbacks(onChanges *[]OnChange, evtName string) {
	for _, o := range *onChanges {
		if c.getIsIgnored(evtName, &o.ExcludedPatterns) {
			continue
		}
		err := o.Func(evtName)
		if err != nil {
			c.Logger.Errorf("error running extension callback: %v", err)
		}
	}
}
