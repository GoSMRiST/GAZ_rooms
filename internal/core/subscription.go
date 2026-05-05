package core

type Subscription string

const (
	SubscriptionDefault Subscription = "default"
	SubscriptionVip     Subscription = "vip"
	SubscriptionSuper   Subscription = "super"
)

var RoomCreationLimits = map[Subscription]int{
	SubscriptionDefault: 3,
	SubscriptionVip:     5,
	SubscriptionSuper:   10,
}

var PeopleLimits = map[Subscription]int{
	SubscriptionDefault: 10,
	SubscriptionVip:     20,
	SubscriptionSuper:   50,
}

const MaxPeopleLimit = 50
