package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/op/go-logging"
	"github.com/textileio/textile-go/central/models"
	tcore "github.com/textileio/textile-go/core"
	"github.com/textileio/textile-go/crypto"
	"github.com/textileio/textile-go/net"
	"github.com/textileio/textile-go/repo"
	"github.com/textileio/textile-go/wallet"
	"github.com/textileio/textile-go/wallet/thread"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	libp2pc "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

var log = logging.MustGetLogger("mobile")

// Message is a generic go -> bridge message structure
type Event struct {
	Name    string `json:"name"`
	Payload string `json:"payload"`
}

// newEvent transforms an event name and structured data in Event
func newEvent(name string, payload map[string]interface{}) *Event {
	event := &Event{Name: name}
	jsonb, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("error creating event data json: %s", err)
	}
	event.Payload = string(jsonb)
	return event
}

// Messenger is used to inform the bridge layer of new data waiting to be queried
type Messenger interface {
	Notify(event *Event)
}

// Wrapper is the object exposed in the frameworks
type Wrapper struct {
	RepoPath  string
	messenger Messenger
}

// NodeConfig is used to configure the mobile node
type NodeConfig struct {
	RepoPath      string
	CentralApiURL string
	LogLevel      string
	LogFiles      bool
}

// NewNode is the mobile entry point for creating a node
// NOTE: logLevel is one of: CRITICAL ERROR WARNING NOTICE INFO DEBUG
func NewNode(config *NodeConfig, messenger Messenger) (*Wrapper, error) {
	var m Mobile
	return m.NewNode(config, messenger)
}

// Mobile is the name of the framework (must match package name)
type Mobile struct{}

// Blocks is a wrapper around a list of Blocks, which makes decoding json from a little cleaner
// on the mobile side
type Blocks struct {
	Items []repo.Block `json:"items"`
}

// tmp while central does not proxy the remote ipfs cluster
const RemoteIPFSApi = "https://ipfs.textile.io/api/v0"

// Create a gomobile compatible wrapper around TextileNode
func (m *Mobile) NewNode(config *NodeConfig, messenger Messenger) (*Wrapper, error) {
	ll, err := logging.LogLevel(config.LogLevel)
	if err != nil {
		ll = logging.INFO
	}
	cconfig := tcore.NodeConfig{
		LogLevel: ll,
		LogFiles: config.LogFiles,
		WalletConfig: wallet.Config{
			RepoPath:   config.RepoPath,
			CentralAPI: config.CentralApiURL,
			IsMobile:   true,
		},
	}
	node, err := tcore.NewNode(cconfig)
	if err != nil {
		return nil, err
	}
	tcore.Node = node

	return &Wrapper{RepoPath: config.RepoPath, messenger: messenger}, nil
}

// Start the mobile node
func (w *Wrapper) Start() error {
	online, err := tcore.Node.StartWallet()
	if err != nil {
		if err == wallet.ErrStarted {
			return nil
		}
		return err
	}

	go func() {
		<-online
		// join existing threads
		for _, thrd := range tcore.Node.Wallet.Threads() {
			w.subscribe(thrd)
		}

		// notify UI we're ready
		w.messenger.Notify(newEvent("onOnline", map[string]interface{}{}))

		// publish
		tcore.Node.Wallet.PublishThreads()
	}()

	return nil
}

// Stop the mobile node
func (w *Wrapper) Stop() error {
	if err := tcore.Node.StopWallet(); err != nil && err != wallet.ErrStopped {
		return err
	}
	return nil
}

// SignUpWithEmail creates an email based registration and calls core signup
func (w *Wrapper) SignUpWithEmail(username string, password string, email string, referral string) error {
	// build registration
	reg := &models.Registration{
		Username: username,
		Password: password,
		Identity: &models.Identity{
			Type:  models.EmailAddress,
			Value: email,
		},
		Referral: referral,
	}
	return tcore.Node.Wallet.SignUp(reg)
}

// SignIn build credentials and calls core SignIn
func (w *Wrapper) SignIn(username string, password string) error {
	// build creds
	creds := &models.Credentials{
		Username: username,
		Password: password,
	}
	return tcore.Node.Wallet.SignIn(creds)
}

// SignOut calls core SignOut
func (w *Wrapper) SignOut() error {
	return tcore.Node.Wallet.SignOut()
}

// IsSignedIn calls core IsSignedIn
func (w *Wrapper) IsSignedIn() bool {
	si, _ := tcore.Node.Wallet.IsSignedIn()
	return si
}

// GetId calls core GetId
func (w *Wrapper) GetId() (string, error) {
	return tcore.Node.Wallet.GetId()
}

// GetIPFSPeerId returns the wallet's ipfs peer id
func (w *Wrapper) GetIPFSPeerId() (string, error) {
	return tcore.Node.Wallet.GetIPFSPeerId()
}

// GetUsername calls core GetUsername
func (w *Wrapper) GetUsername() (string, error) {
	return tcore.Node.Wallet.GetUsername()
}

// GetAccessToken calls core GetAccessToken
func (w *Wrapper) GetAccessToken() (string, error) {
	return tcore.Node.Wallet.GetAccessToken()
}

// AddThread adds a new thread with the given name
func (w *Wrapper) AddThread(name string, mnemonic string) error {
	var mnem *string
	if mnemonic != "" {
		mnem = &mnemonic
	}
	_, _, err := tcore.Node.Wallet.AddThreadWithMnemonic(name, mnem)
	if err == wallet.ErrThreadExists || err == wallet.ErrThreadLoaded {
		return nil
	}
	return err
}

// AddPhoto adds a photo by path and shares it to the default thread
func (w *Wrapper) AddPhoto(path string, threadName string, caption string) (*net.MultipartRequest, error) {
	thrd := tcore.Node.Wallet.GetThreadByName(threadName)
	if thrd == nil {
		return nil, errors.New(fmt.Sprintf("could not find thread: %s", threadName))
	}
	added, err := tcore.Node.Wallet.AddPhoto(path)
	if err != nil {
		return nil, err
	}
	shared, err := thrd.AddPhoto(added.Id, caption, added.Key)
	if err != nil {
		return nil, err
	}

	// pin to remote
	url := fmt.Sprintf("%s/add?wrap-with-directory=true", RemoteIPFSApi)
	status, err := shared.RemoteRequest.Send(url)
	if err != nil {
		return nil, err
	}
	log.Debugf("pinned block to remote (status %s)", status)

	// let the OS handle the large upload
	return added.RemoteRequest, nil
}

// SharePhoto adds an existing photo to a new thread
func (w *Wrapper) SharePhoto(id string, threadName string, caption string) (string, error) {
	block, err := tcore.Node.Wallet.GetBlockByTarget(id)
	if err != nil {
		return "", err
	}
	fromThread := tcore.Node.Wallet.GetThread(block.ThreadPubKey)
	if fromThread == nil {
		return "", errors.New(fmt.Sprintf("could not find thread %s", block.ThreadPubKey))
	}
	toThread := tcore.Node.Wallet.GetThreadByName(threadName)
	if toThread == nil {
		return "", errors.New(fmt.Sprintf("could not find thread named %s", threadName))
	}
	key, err := fromThread.Decrypt(block.TargetKey)
	if err != nil {
		return "", err
	}

	// TODO: owner challenge
	shared, err := toThread.AddPhoto(id, caption, key)
	if err != nil {
		return "", err
	}

	// pin to remote
	url := fmt.Sprintf("%s/add?wrap-with-directory=true", RemoteIPFSApi)
	status, err := shared.RemoteRequest.Send(url)
	if err != nil {
		return "", err
	}
	log.Debugf("pinned block to remote (status %s)", status)

	return shared.Id, nil
}

// GetPhotoBlocks returns thread photo blocks with json encoding
func (w *Wrapper) GetPhotoBlocks(offsetId string, limit int, threadName string) (string, error) {
	thrd := tcore.Node.Wallet.GetThreadByName(threadName)
	if thrd == nil {
		return "", errors.New(fmt.Sprintf("thread not found: %s", threadName))
	}

	// use this opportunity to post head
	if tcore.Node.Wallet.Online() {
		go thrd.PostHead()
	}

	blocks := &Blocks{thrd.Blocks(offsetId, limit)}
	jsonb, err := json.Marshal(blocks)
	if err != nil {
		log.Errorf("error marshaling json: %s", err)
		return "", err
	}

	return string(jsonb), nil
}

// GetBlockData calls GetBlockDataBase64 on a thread
func (w *Wrapper) GetBlockData(id string, path string) (string, error) {
	block, err := tcore.Node.Wallet.GetBlock(id)
	if err != nil {
		log.Errorf("could not find block %s: %s", id, err)
		return "", err
	}
	thrd := tcore.Node.Wallet.GetThread(block.ThreadPubKey)
	if thrd == nil {
		err := errors.New(fmt.Sprintf("could not find thread: %s", block.ThreadPubKey))
		log.Error(err.Error())
		return "", err
	}

	return thrd.GetBlockDataBase64(fmt.Sprintf("%s/%s", id, path), block)
}

// GetFileData calls GetFileDataBase64 on a thread
func (w *Wrapper) GetFileData(id string, path string) (string, error) {
	block, err := tcore.Node.Wallet.GetBlockByTarget(id)
	if err != nil {
		log.Errorf("could not find block for target %s: %s", id, err)
		return "", err
	}
	thrd := tcore.Node.Wallet.GetThread(block.ThreadPubKey)
	if thrd == nil {
		err := errors.New(fmt.Sprintf("could not find thread: %s", block.ThreadPubKey))
		log.Error(err.Error())
		return "", err
	}

	return thrd.GetFileDataBase64(fmt.Sprintf("%s/%s", id, path), block)
}

// PairDevice publishes this node's secret key to another node,
// which is listening at it's own peer id
func (w *Wrapper) PairDevice(pkb64 string) (string, error) {
	if !tcore.Node.Wallet.Online() {
		return "", wallet.ErrOffline
	}
	log.Info("pairing with a new device...")

	pkb, err := libp2pc.ConfigDecodeKey(pkb64)
	if err != nil {
		return "", err
	}
	pk, err := libp2pc.UnmarshalPublicKey(pkb)
	if err != nil {
		return "", err
	}

	// we invite the desktop to _read and write_ to our default album
	defaultThread := tcore.Node.Wallet.GetThreadByName("default")
	if defaultThread == nil {
		err = errors.New("default thread not found")
		log.Error(err.Error())
		return "", err
	}
	// encypt thread secret key with the desktop's pub key
	secret, err := defaultThread.PrivKey.Bytes()
	if err != nil {
		return "", err
	}
	secretcypher, err := crypto.Encrypt(pk, secret)
	if err != nil {
		return "", err
	}

	// get the topic to pair with from the pub key
	peerID, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}
	topic := peerID.Pretty()

	// finally, publish the encrypted phrase
	err = tcore.Node.Wallet.Publish(topic, secretcypher)
	if err != nil {
		return "", err
	}
	log.Infof("published key phrase to device: %s", topic)

	return topic, nil
}

// subscribe to thread and pass updates to messenger
func (w *Wrapper) subscribe(thrd *thread.Thread) {
	datac := make(chan thread.Update)
	go thrd.Subscribe(datac)
	go func() {
		for {
			select {
			case update, ok := <-datac:
				if !ok {
					return
				}
				w.messenger.Notify(newEvent("onThreadUpdate", map[string]interface{}{
					"id":        update.Id,
					"thread":    update.Thread,
					"thread_id": update.ThreadID,
				}))
			}
		}
	}()
}
