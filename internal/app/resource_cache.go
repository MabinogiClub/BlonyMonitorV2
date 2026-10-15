package app

import (
	"sync"

	"blonymonitorv2/db"
)

// mabidata.db 里的技能 / 种族 / 状态是静态字典表，但它们没有索引，连接池又只有 1 个连接
// （见 db.InitDB 的 SetMaxOpenConns(1)）。而伤害统计每次刷新都会为每个
// (target, attacker, skill) 组合查一次库，实战可达每秒上万次：
// CPU 被打满，并在持有 a.mu 读锁的情况下卡住数秒到数十秒，把需要写锁的数据包处理饿死
// （表现为 UI 卡顿 + 数据包处理滞后几十分钟）。
//
// 这里给这几个字典查询加一层进程内缓存：miss 时回源一次并记住结果（含"数据库中不存在"
// 这一情况，避免对未知 ID 反复查库）。字典表只在 mabidata.db 更新时变化，重启即刷新。

type cachedResourceName struct {
	name string
	// found 表示数据库中确有该条目（本地名或英文名非空）；查询未命中时为 false。
	found bool
}

type resourceNameCache struct {
	mu    sync.RWMutex
	items map[int]cachedResourceName
	load  func(id int) cachedResourceName
}

func newResourceNameCache(load func(id int) cachedResourceName) *resourceNameCache {
	return &resourceNameCache{items: make(map[int]cachedResourceName), load: load}
}

// lookup 返回名称与"数据库中是否存在该条目"的标志，两者都会被缓存。
// 锁只保护缓存本身，不参与 a.mu，因此可以安全地在持读锁的调用链里使用。
func (c *resourceNameCache) lookup(id int) (string, bool) {
	c.mu.RLock()
	item, ok := c.items[id]
	c.mu.RUnlock()
	if ok {
		return item.name, item.found
	}

	item = c.load(id)

	c.mu.Lock()
	c.items[id] = item
	c.mu.Unlock()
	return item.name, item.found
}

// name 供只关心名称的调用方使用。
func (c *resourceNameCache) name(id int) string {
	name, _ := c.lookup(id)
	return name
}

var (
	skillNameCache = newResourceNameCache(func(id int) cachedResourceName {
		info := db.NewSkillInfo(id)
		return cachedResourceName{
			name:  info.GetName(),
			found: info.SkillLocalName != "" || info.SkillEngName != "",
		}
	})

	raceNameCache = newResourceNameCache(func(id int) cachedResourceName {
		info := db.NewRace(id)
		return cachedResourceName{
			name:  info.GetName(),
			found: info.LocalName != "" || info.EnglishName != "" || info.ClassName != "",
		}
	})

	conditionNameCache = newResourceNameCache(func(id int) cachedResourceName {
		info := db.NewCharacterCondition(id)
		return cachedResourceName{
			name:  info.GetName(),
			found: info.ConditionLocalName != "" || info.ConditionEngName != "",
		}
	})
)
