package mylog

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var LogFile = flag.String("log.file", "", "save log file path")
var logNum = flag.Int("log.num", 10000, " the loginfo number of log file")
var logFileNum = flag.Int("log.filenum", 3, " the max num of log file to save")

const (
	LOG_DEPTH = 3
	LDEBUG    = iota
	LINFO     //1
	LNOTICE
	LWARNING
	LERROR
)

var loglevel int
var MyLogInfoNum uint64 = 0
var LogInfoThreshold uint64 = 0
var logLock sync.Mutex
var logFileChan chan string
var logFileFlag bool //false means don't create log
var Day int

var lf *os.File

func redirectStderr(f *os.File) {
	err := syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
}

func fileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func getLogLevel(logLevel string) (int, error) {
	switch logLevel {
	case "debug":
		return LDEBUG, nil
	case "info":
		return LINFO, nil
	case "notice":
		return LNOTICE, nil
	case "warn":
		return LWARNING, nil
	case "error":
		return LERROR, nil
	default:
		return LINFO, fmt.Errorf("unknow log leve %s, must be %s", logLevel, ShowSupportLevels())
	}
}

func SetLogLevel(log_level string) error {
	Printf("---------SetLogLevel=%s-------------\n", log_level)
	level, err := getLogLevel(log_level)
	if err != nil {
		return err
	}
	loglevel = level
	return nil
}

func ShowSupportLevels() string {
	return "debug, info, notice, warn, error"
}

func InitLog(log_level string, logFile string) {
	if logFile != "" {
		*LogFile = logFile
	}
	loglevel, _ = getLogLevel(log_level)
	LogInfoThreshold = uint64(*logNum)
	logFileChan = make(chan string, *logFileNum)
	logFileFlag = false
	createLogFile()
}

func Close() {
	if lf != nil {
		lf.Close()
	}
}

func init() {

}

func createLogFile() {
	logfile := *LogFile
	log.Printf("=======original log.file is : %s, try to create it, logFileNum=%d, logNum=%d===========\n", logfile, *logFileNum, *logNum)
	var err error
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	if logfile != "" {
		if err := os.MkdirAll(path.Dir(logfile), 0644); err != nil {
			log.Fatalln("os.MkdirAll error: ", err)
		}
		//if fileExist(logfile)
		{
			t := time.Now()
			year, month, day := t.Date()
			filename := path.Base(logfile)
			fileSuffix := path.Ext(filename) //获取文件后缀
			filename_olny := strings.TrimSuffix(filename, fileSuffix)
			logfile = path.Dir(logfile) + string(os.PathSeparator) + filename_olny +
				fmt.Sprintf("_%d-%d-%d", year, month, day) + fileSuffix
			log.Printf("generate new log file name: %s\n", logfile)
			Day = day
		}

		lf, err = os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatalln("open log file error: ", err)
		}
		if len(logFileChan) >= *logFileNum {
			log.Printf("len(logFileChan):%d, max log file num is %d, need to del the oldest log file\n", len(logFileChan), *logFileNum)
			oldestFile := <-logFileChan
			if err := os.Remove(oldestFile); err != nil {
				log.Printf("fail, del the oldest file:%s, err:%s\n", oldestFile, err.Error())
			} else {
				log.Printf("success, del the oldest file:%s\n", oldestFile)
			}
		}

		if len(logFileChan) >= *logFileNum {
			log.Panicf(" error about mylog, len(logFileChan)=%d, *logFileNum=%d\n", len(logFileChan), *logFileNum)
		}
		logFileChan <- logfile
		log.Printf("save log file:%s to chan ok\n", logfile)

		log.Printf("create log file  %s success, now set log output to the file\n", logfile)
		redirectStderr(lf)
		log.SetOutput(lf)

		logFileFlag = true

		nextDay := getNextDay()
		log.Printf("next day =%v \n", nextDay)
		time.AfterFunc(nextDay.Sub(time.Now()), createLogFile)
	}
}

func getNextDay() time.Time {
	now := time.Now()
	year, month, day := now.Date()
	return time.Date(year, month, day+1, 0, 0, 1, 0, now.Location())
}

func NewLogFile() {
	logLock.Lock()
	LogInfoThreshold += uint64(*logNum)
	if logFileFlag == false {
		logLock.Unlock()
		return
	}
	logFileFlag = false
	logLock.Unlock()

	go createLogFile()
}

func putToLog(level int, pre string, format string, a ...interface{}) {
	if loglevel <= level {
		pre_str := fmt.Sprintf("[%s %d] ", pre, MyLogInfoNum)
		log.Output(LOG_DEPTH, fmt.Sprintf(pre_str+format, a...))
		atomic.AddUint64(&MyLogInfoNum, 1)
		// if *LogFile != "" && Day != time.Now().Day() {
		// 	log.Printf("====Day(%d) != time.Now().Day() =====\n", Day)
		// 	NewLogFile()
		// }
		// if MyLogInfoNum > LogInfoThreshold {
		// 	NewLogFile()
		// }
	}
}

func Printf(format string, a ...interface{}) {
	log.Output(2, fmt.Sprintf(format, a...))
}

func Debug(format string, a ...interface{}) {
	putToLog(LDEBUG, "Debug", format, a...)
}

func Info(format string, a ...interface{}) {
	putToLog(LINFO, "Info", format, a...)
}

func Notice(format string, a ...interface{}) {
	putToLog(LNOTICE, "Notice", format, a...)
}

func Warning(format string, a ...interface{}) {
	putToLog(LWARNING, "Warning", format, a...)
}

func Error(format string, a ...interface{}) {
	putToLog(LERROR, "Error", format, a...)
}
