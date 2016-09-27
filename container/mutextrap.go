package container

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MUTEX_TRAP_TIMEOUT        = 15 * time.Second
	MUTEX_TRAP_MAX_TRIP_COUNT = 3
	MUTEX_TRAP_ADDR_FILE      = "/mutextrap"
)

func getReportAddr() string {
	addr, err := ioutil.ReadFile(MUTEX_TRAP_ADDR_FILE)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(addr))
}

func sendReport(text string) {
	fmt.Fprintf(os.Stderr, "MutexTrap hit:\n%s\n", text)
	addr := getReportAddr()
	if addr == "" {
		return
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MutexTrap -- error sending report: %s\n", err)
		return
	}
	_, err = fmt.Fprintf(conn, "MutexTrap hit:\n%s\n", text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MutexTrap -- error sending report: %s\n", err)
	}
	conn.Close()
}

func getStackTrace() string {
	var buf bytes.Buffer
	pprof.Lookup("goroutine").WriteTo(&buf, 1)
	return fmt.Sprintf("Time: %s\nStack:\n%s\n----- all goroutines -----\n%s", time.Now(), debug.Stack(), buf.String())
}

type MutexTrap struct {
	mtx       sync.Mutex
	stack     string
	tripCount int32
}

func (mt *MutexTrap) Lock() {
	timer := time.NewTimer(MUTEX_TRAP_TIMEOUT)
	done := make(chan struct{})
	stack := getStackTrace()
	go func() {
		select {
		case <-timer.C:
			if atomic.AddInt32(&mt.tripCount, 1) > MUTEX_TRAP_MAX_TRIP_COUNT {
				return
			}
			sendReport(fmt.Sprintf("***************\n*** CURRENT ***\n%s\n*** THE CULPRIT ***\n%s", stack, mt.stack))
		case <-done:
			timer.Stop()
		}
	}()
	mt.mtx.Lock()
	close(done)
	mt.stack = stack
}

func (mt *MutexTrap) Unlock() {
	mt.stack = ""
	mt.mtx.Unlock()
}
