package app

import (
	"context"
	"time"
)

const farmSoundPriorityWindow = 120 * time.Millisecond

type farmSoundKind uint8

const (
	farmSoundIdle farmSoundKind = iota
	farmSoundReadyDelay
	farmSoundReady
	farmSoundSpecial
)

type farmSoundPlayer struct {
	ctx          context.Context
	requests     chan farmSoundKind
	getVolume    func() int
	play         func(string, int) time.Duration
	priorityWait time.Duration
}

func newFarmSoundPlayer(ctx context.Context, getVolume func() int) *farmSoundPlayer {
	return newFarmSoundPlayerWithPlayback(ctx, getVolume, farmSoundPriorityWindow, playFarmSoundNow)
}

func newFarmSoundPlayerWithPlayback(
	ctx context.Context,
	getVolume func() int,
	priorityWait time.Duration,
	play func(string, int) time.Duration,
) *farmSoundPlayer {
	player := &farmSoundPlayer{
		ctx:          ctx,
		requests:     make(chan farmSoundKind, len(farmSlotDefinitions)*2),
		getVolume:    getVolume,
		play:         play,
		priorityWait: priorityWait,
	}
	go player.run()
	return player
}

func (p *farmSoundPlayer) NotifyReady() {
	p.notify(farmSoundReady)
}

func (p *farmSoundPlayer) NotifySpecial() {
	p.notify(farmSoundSpecial)
}

func (p *farmSoundPlayer) notify(kind farmSoundKind) {
	select {
	case p.requests <- kind:
	case <-p.ctx.Done():
	}
}

func (p *farmSoundPlayer) run() {
	state := farmSoundIdle
	pendingReady := false
	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
	}
	setTimer := func(duration time.Duration) {
		stopTimer()
		if timer == nil {
			timer = time.NewTimer(duration)
		} else {
			timer.Reset(duration)
		}
		timerC = timer.C
	}

	play := func(kind farmSoundKind) {
		filename := "农作物成熟.wav"
		if kind == farmSoundSpecial {
			filename = "这是一颗高级种子.wav"
		}
		duration := p.play(filename, p.getVolume())
		state = kind
		if duration > 0 {
			setTimer(duration)
			return
		}
		state = farmSoundIdle
		stopTimer()
	}
	scheduleReady := func() {
		state = farmSoundReadyDelay
		setTimer(p.priorityWait)
	}

	defer stopTimer()
	for {
		select {
		case <-p.ctx.Done():
			return
		case request := <-p.requests:
			switch request {
			case farmSoundSpecial:
				switch state {
				case farmSoundSpecial:
					continue
				case farmSoundReadyDelay, farmSoundReady:
					pendingReady = true
				}
				play(farmSoundSpecial)
				if state == farmSoundIdle && pendingReady {
					pendingReady = false
					scheduleReady()
				}
			case farmSoundReady:
				switch state {
				case farmSoundIdle:
					scheduleReady()
				case farmSoundSpecial:
					pendingReady = true
				}
			}
		case <-timerC:
			timerC = nil
			switch state {
			case farmSoundReadyDelay:
				play(farmSoundReady)
			case farmSoundReady:
				state = farmSoundIdle
			case farmSoundSpecial:
				state = farmSoundIdle
				if pendingReady {
					pendingReady = false
					scheduleReady()
				}
			}
		}
	}
}
