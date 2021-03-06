package cmd

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/textileio/textile-go/core"
	"github.com/textileio/textile-go/wallet/thread"
	"gopkg.in/abiosoft/ishell.v2"
)

func ListThreads(c *ishell.Context) {
	threads := core.Node.Wallet.Threads()
	if len(threads) == 0 {
		c.Println("no threads found")
	} else {
		c.Println(fmt.Sprintf("found %v threads", len(threads)))
	}

	blue := color.New(color.FgHiBlue).SprintFunc()
	for _, thrd := range threads {
		mem := "disabled"
		if thrd.Listening() {
			mem = "enabled"
		}
		c.Println(blue(fmt.Sprintf("name: %s, id: %s, status: %s", thrd.Name, thrd.Id, mem)))
	}
}

func CreateThread(c *ishell.Context) {
	if len(c.Args) == 0 {
		c.Err(errors.New("missing thread name"))
		return
	}
	name := c.Args[0]

	c.Print("key pair mnemonic phrase (optional): ")
	mnemonics := c.ReadLine()
	var mnemonic *string
	if mnemonics != "" {
		mnemonic = &mnemonics
	}

	thrd, mnem, err := core.Node.Wallet.AddThreadWithMnemonic(name, mnemonic)
	if err != nil {
		c.Err(err)
		return
	}

	Subscribe(c, thrd)

	cyan := color.New(color.FgCyan).SprintFunc()
	c.Println(cyan(fmt.Sprintf("created thread #%s", name)))
	c.Println(cyan(fmt.Sprintf("mnemonic phrase: %s", mnem)))
}

func EnableThread(c *ishell.Context) {
	if len(c.Args) == 0 {
		c.Err(errors.New("missing thread name"))
		return
	}
	name := c.Args[0]

	thrd := core.Node.Wallet.GetThreadByName(name)
	if thrd == nil {
		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", name)))
		return
	}

	if thrd.Listening() {
		c.Printf("already enabled: %s\n", thrd.Id)
		return
	}

	Subscribe(c, thrd)

	c.Printf("ok, now enabled: %s\n", thrd.Id)
}

func DisableAlbum(c *ishell.Context) {
	if len(c.Args) == 0 {
		c.Err(errors.New("missing thread name"))
		return
	}
	name := c.Args[0]

	thrd := core.Node.Wallet.GetThreadByName(name)
	if thrd == nil {
		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", name)))
		return
	}

	if !thrd.Listening() {
		c.Printf("already disabled: %s\n", thrd.Id)
		return
	}

	thrd.Unsubscribe()
	<-thrd.LeftCh

	c.Printf("ok, now disabled: %s\n", thrd.Id)
}

func PublishThread(c *ishell.Context) {
	if len(c.Args) == 0 {
		c.Err(errors.New("missing thread name"))
		return
	}
	name := c.Args[0]

	thrd := core.Node.Wallet.GetThreadByName(name)
	if thrd == nil {
		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", name)))
		return
	}
	head, err := thrd.GetHead()
	if err != nil {
		c.Err(err)
		return
	}
	if head == "" {
		head = "ping"
	}
	err = thrd.PostHead()
	if err != nil {
		c.Err(err)
		return
	}

	blue := color.New(color.FgHiBlue).SprintFunc()
	c.Println(blue(fmt.Sprintf("published %s to %s", head, thrd.Id)))
}

func ListThreadPeers(c *ishell.Context) {
	if len(c.Args) == 0 {
		c.Err(errors.New("missing thread name"))
		return
	}
	name := c.Args[0]

	thrd := core.Node.Wallet.GetThreadByName(name)
	if thrd == nil {
		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", name)))
		return
	}

	peers := thrd.Peers()
	if len(peers) == 0 {
		c.Println(fmt.Sprintf("no peers found in: %s", name))
	} else {
		c.Println(fmt.Sprintf("found %v peers in: %s", len(peers), name))
	}

	green := color.New(color.FgHiGreen).SprintFunc()
	for _, peer := range peers {
		c.Println(green(peer))
	}
}

func Subscribe(shell ishell.Actions, thrd *thread.Thread) {
	cyan := color.New(color.FgCyan).SprintFunc()
	datac := make(chan thread.Update)
	go thrd.Subscribe(datac)
	go func() {
		for {
			select {
			case update, ok := <-datac:
				if !ok {
					return
				}
				msg := fmt.Sprintf("\nnew photo %s in thread %s\n", update.Id, update.Thread)
				shell.ShowPrompt(false)
				shell.Printf(cyan(msg))
				shell.ShowPrompt(true)
			}
		}
	}()
}
