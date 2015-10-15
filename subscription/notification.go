package subscription

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/oursky/skygear/oddb"
	"github.com/oursky/skygear/pubsub"
	"github.com/oursky/skygear/push"
)

// Notice encapsulates the information sent to subscribers when the content of
// a subscription has changed.
type Notice struct {
	// SeqNum is a strictly increasing number between notice
	SeqNum         uint64
	SubscriptionID string
	Event          oddb.RecordHookEvent
	Record         *oddb.Record
}

// Notifier is the interface implemented by an object that knows how to deliver
// a Notice to a device.
type Notifier interface {
	// CanNotify returns whether the Notifier can send notice to the device.
	CanNotify(device oddb.Device) bool

	// Notify sends an notice to the device.
	Notify(device oddb.Device, notice Notice) error
}

type pushNotifier struct {
	sender push.Sender
}

// NewPushNotifier returns an Notifier which sends Notice
// using the given push.Sender.
func NewPushNotifier(sender push.Sender) Notifier {
	return &pushNotifier{sender}
}

func (notifier *pushNotifier) CanNotify(device oddb.Device) bool {
	return device.Type == "ios"
}

func (notifier *pushNotifier) Notify(device oddb.Device, notice Notice) error {
	customMap := map[string]interface{}{
		"aps": map[string]interface{}{
			"content_available": 1,
		},
		"_ourd": map[string]interface{}{
			"seq-num":         notice.SeqNum,
			"subscription-id": notice.SubscriptionID,
		},
	}

	return notifier.sender.Send(push.MapMapper(customMap), device.Token)
}

type hubNotifier pubsub.Hub

// NewHubNotifier returns an Notifier which sends Notice thru the supplied
// hub. The notice will be sent via the channel name "_sub_[DEVICE_ID]".
func NewHubNotifier(hub *pubsub.Hub) Notifier {
	return (*hubNotifier)(hub)
}

func (n *hubNotifier) CanNotify(device oddb.Device) bool {
	return true
}

func (n *hubNotifier) Notify(device oddb.Device, notice Notice) error {
	data, err := json.Marshal(struct {
		SeqNum         uint64 `json:"seq-num"`
		SubscriptionID string `json:"subscription-id"`
	}{notice.SeqNum, notice.SubscriptionID})

	if err == nil {
		(*pubsub.Hub)(n).Broadcast <- pubsub.Parcel{
			Channel: fmt.Sprintf("_sub_%s", device.ID),
			Data:    data,
		}
	}

	return err
}

type multiNotifier []Notifier

// NewMultiNotifier returns a Notifier which sends Notice to multiple
// underlying Notifiers
func NewMultiNotifier(notifiers ...Notifier) Notifier {
	return multiNotifier(notifiers)
}

func (ns multiNotifier) CanNotify(device oddb.Device) bool {
	return true
}

func (ns multiNotifier) Notify(device oddb.Device, notice Notice) error {
	n := len(ns)

	errCh := make(chan error)
	for _, notifier := range ns {
		notifier := notifier
		if notifier.CanNotify(device) {
			go func() {
				errCh <- notifier.Notify(device, notice)
			}()
		}
	}

	var lasterr error
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			lasterr = err
			log.WithFields(log.Fields{
				"device": device,
				"notice": notice,
				"err":    err,
			}).Errorf("multi-notifier: failed to send notice")
		}
	}

	return lasterr
}
