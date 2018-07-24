package tail

import (
	"regexp"

	"github.com/golang/glog"

	// "github.com/fsnotify/fsnotify"
	"github.com/fsnotify/fsnotify"
	"github.com/hpcloud/tail"
	"github.com/wanghengwei/monclient/filenotify"
)

// Util 用来执行tail的工具类
type Util struct {
	NewData      chan *Info
	watcher      filenotify.FileWatcher
	namePattern  *regexp.Regexp
	watchedFiles map[string]*tail.Tail
}

// New 对paths进行后台tail
func New(folder string, namePattern string) (*Util, error) {
	w, err := filenotify.New()
	if err != nil {
		return nil, err
	}
	err = w.Add(folder)
	if err != nil {
		return nil, err
	}

	p, err := regexp.Compile(namePattern)
	if err != nil {
		return nil, err
	}

	t := &Util{
		watcher:      w,
		namePattern:  p,
		watchedFiles: make(map[string]*tail.Tail),
		NewData:      make(chan *Info, 32),
	}

	go t.watchFolder()

	return t, nil
}

// Close close it
func (u *Util) Close() error {
	close(u.NewData)
	err := u.watcher.Close()
	return err
}

func (u *Util) matchName(name string) bool {
	return u.namePattern.MatchString(name)
}

func (u *Util) transLines(t *tail.Tail, fp string) {
	for l := range t.Lines {
		u.NewData <- &Info{FilePath: fp, Line: l.Text}
	}
	glog.Infof("end goroutine for tail file: %s\n", fp)
}

func (u *Util) watchFolder() {
	glog.Infof("start watching folder\n")
	for {
		select {
		case ev := <-u.watcher.Events():
			if !u.matchName(ev.Name) {
				glog.Infof("file name %s is not interested, skip\n", ev.Name)
				break
			}

			if ev.Op&fsnotify.Create == fsnotify.Create {
				// 创建了一个新文件，应当增加对其的tail
				glog.Infof("add watched file %s\n", ev.Name)

				t, err := tail.TailFile(ev.Name, tail.Config{Follow: true})
				if err != nil {
					glog.Infof("failed to tail file %s\n", ev.Name)
					break
				}

				go u.transLines(t, ev.Name)

				u.watchedFiles[ev.Name] = t
			} else if ev.Op&fsnotify.Write == fsnotify.Write {
				_, ok := u.watchedFiles[ev.Name]
				if ok {
					glog.Infof("ingore watched file %s\n", ev.Name)
					break
				}

				// 已有的文件，新写入了，也加入tail
				glog.Infof("add watched file %s\n", ev.Name)

				t, err := tail.TailFile(ev.Name, tail.Config{Follow: true, Poll: true})
				if err != nil {
					glog.Infof("failed to tail file %s\n", ev.Name)
					break
				}

				go u.transLines(t, ev.Name)

				u.watchedFiles[ev.Name] = t
			} else if ev.Op&fsnotify.Remove == fsnotify.Remove {
				glog.Infof("delete watched file %s\n", ev.Name)

				t, ok := u.watchedFiles[ev.Name]
				if !ok {
					break
				}
				t.Stop()
				t.Cleanup()

				delete(u.watchedFiles, ev.Name)
			} else {
				glog.V(1).Infof("ingore event %v\n", ev)
			}
		case err := <-u.watcher.Errors():
			glog.Warningf("watch failed: %s\n", err)
		}
	}
}

// Info 表示是哪个文件的新数据
type Info struct {
	FilePath string
	Line     string
}
