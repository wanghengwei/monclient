package tail

import (
	"log"
	"regexp"

	"github.com/fsnotify/fsnotify"
	"github.com/hpcloud/tail"
)

// Util 用来执行tail的工具类
type Util struct {
	NewData      chan *Info
	watcher      *fsnotify.Watcher
	namePattern  *regexp.Regexp
	watchedFiles map[string]*tail.Tail
}

// New 对paths进行后台tail
func New(folder string, namePattern string) (*Util, error) {
	w, err := fsnotify.NewWatcher()
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
	log.Printf("end goroutine for tail file: %s\n", fp)
}

func (u *Util) watchFolder() {
	log.Println("start watching...")
	for {
		select {
		case ev := <-u.watcher.Events:
			if !u.matchName(ev.Name) {
				break
			}

			if ev.Op&fsnotify.Create == fsnotify.Create {
				// 创建了一个新文件，应当增加对其的tail
				log.Printf("add watched file %s\n", ev.Name)

				t, err := tail.TailFile(ev.Name, tail.Config{Follow: true})
				if err != nil {
					log.Printf("failed to tail file %s\n", ev.Name)
					break
				}

				go u.transLines(t, ev.Name)

				u.watchedFiles[ev.Name] = t
			} else if ev.Op&fsnotify.Write == fsnotify.Write {
				_, ok := u.watchedFiles[ev.Name]
				if ok {
					log.Printf("ingore watched file %s\n", ev.Name)
					break
				}

				// 已有的文件，新写入了，也加入tail
				log.Printf("add watched file %s\n", ev.Name)

				t, err := tail.TailFile(ev.Name, tail.Config{Follow: true, Poll: true})
				if err != nil {
					log.Printf("failed to tail file %s\n", ev.Name)
					break
				}

				go u.transLines(t, ev.Name)

				u.watchedFiles[ev.Name] = t
			} else if ev.Op&fsnotify.Remove == fsnotify.Remove {
				log.Printf("delete watched file %s\n", ev.Name)

				t, ok := u.watchedFiles[ev.Name]
				if !ok {
					break
				}
				t.Stop()
				t.Cleanup()

				delete(u.watchedFiles, ev.Name)
			}
		case err := <-u.watcher.Errors:
			log.Printf("watch failed: %s\n", err)
		}
	}
}

// Info 表示是哪个文件的新数据
type Info struct {
	FilePath string
	Line     string
}
