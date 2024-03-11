package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nitishm/go-rejson/v4"
	"github.com/redis/go-redis/v9"
)

const (
	RedisNilError        = "redis: nil"
	RedisDefaultLocation = "0.0.0.0:6379"
	RedisKeyPrefix       = "voter:"
)

type cache struct {
	client     *redis.Client
	jsonHelper *rejson.Handler
	context    context.Context
}

// VoterHistory is the struct that represents a single VoterHistory item
type VoterHistory struct {
	PollId   int       `json:"pollId"`
	VoteId   int       `json:"voteId"`
	VoteDate time.Time `json:"voteDate"`
}

// Voter is the struct that represents a single Voter item
type VoterItem struct {
	VoterId     int            `json:"voterId"`
	Name        string         `json:"name"`
	Email       string         `json:"email"`
	VoteHistory []VoterHistory `json:"voteHistory"`
}

type Voter struct {
	cache
}

// New is a constructor function that returns a pointer to a new VoterList struct
// It uses the default Redis URL with NewVoterListWithCacheInstance.
func New() (*Voter, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = RedisDefaultLocation
	}
	log.Println("DEBUG: USING REDIS URL: ", redisURL)
	return NewWithCacheInstance(redisURL)
}

func NewWithCacheInstance(location string) (*Voter, error) {
	client := redis.NewClient(&redis.Options{
		Addr: location,
	})

	ctx := context.Background()

	err := client.Ping(ctx).Err()
	if err != nil {
		fmt.Println("Error connecting to redis" + err.Error() + "cache might not be available, continuing...")
	}

	jsonHelper := rejson.NewReJSONHandler()
	jsonHelper.SetGoRedisClientWithContext(ctx, client)

	return &Voter{
		cache: cache{
			client:     client,
			jsonHelper: jsonHelper,
			context:    ctx,
		},
	}, nil
}

//------------------------------------------------------------
// REDIS HELPERS
//------------------------------------------------------------

// We will use this later, you can ignore for now
func isRedisNilError(err error) bool {
	return errors.Is(err, redis.Nil) || err.Error() == RedisNilError
}

// In redis, our keys will be strings, they will look like
// todo:<number>.  This function will take an integer and
// return a string that can be used as a key in redis
func redisKeyFromId(id int) string {
	return fmt.Sprintf("%s%d", RedisKeyPrefix, id)
}

// getAllKeys will return all keys in the database that match the prefix
// used in this application - RedisKeyPrefix.  It will return a string slice
// of all keys.  Used by GetAll and DeleteAll
func (vl *Voter) getAllKeys() ([]string, error) {
	key := fmt.Sprintf("%s*", RedisKeyPrefix)
	return vl.cache.client.Keys(vl.context, key).Result()
}

// Helper to return a ToDoItem from redis provided a key
func (vl *Voter) getVoterFromRedis(voterID string, voterItem *VoterItem) error {

	//Lets query redis for the item, note we can return parts of the
	//json structure, the second parameter "." means return the entire
	//json structure
	itemObject, err := vl.jsonHelper.JSONGet(voterID, ".")
	if err != nil {
		return err
	}

	//JSONGet returns an "any" object, or empty interface,
	//we need to convert it to a byte array, which is the
	//underlying type of the object, then we can unmarshal
	//it into our ToDoItem struct
	err = json.Unmarshal(itemObject.([]byte), voterItem)
	if err != nil {
		return err
	}

	return nil
}

// AddVoter adds a new voter to the database
func (vl *Voter) AddVoter(voterItem VoterItem) error {

	//Before we add an item to the DB, lets make sure
	//it does not exist, if it does, return an error
	redisKey := redisKeyFromId(voterItem.VoterId)
	var existingItem VoterItem
	if err := vl.getVoterFromRedis(redisKey, &existingItem); err == nil {
		return errors.New("voter already exists")
	}

	//Add item to database with JSON Set
	if _, err := vl.jsonHelper.JSONSet(redisKey, ".", voterItem); err != nil {
		return err
	}

	//If everything is ok, return nil for the error
	return nil
}

// DeleteVoter deletes a voter from the database
func (vl *Voter) DeleteVoter(id int) error {

	pattern := redisKeyFromId(id)
	numDeleted, err := vl.client.Del(vl.context, pattern).Result()
	if err != nil {
		return err
	}
	if numDeleted == 0 {
		return errors.New("attempted to delete non-existent voterr")
	}

	return nil
}

// DeleteAll deletes all voters from the database
func (vl *Voter) DeleteAll() (int, error) {
	keyList, err := vl.getAllKeys()
	if err != nil {
		return 0, err
	}

	//Notice how we can deconstruct the slice into a variadic argument
	//for the Del function by using the ... operator
	numDeleted, err := vl.client.Del(vl.context, keyList...).Result()
	return int(numDeleted), err
}

// UpdateVoter updates a voter in the database
func (vl *Voter) UpdateVoter(voterItem VoterItem) error {

	//Before we add an item to the DB, lets make sure
	//it does not exist, if it does, return an error
	redisKey := redisKeyFromId(voterItem.VoterId)
	var existingItem VoterItem
	if err := vl.getVoterFromRedis(redisKey, &existingItem); err != nil {
		return errors.New("voter does not exist")
	}

	//Add item to database with JSON Set.  Note there is no update
	//functionality, so we just overwrite the existing item
	if _, err := vl.jsonHelper.JSONSet(redisKey, ".", voterItem); err != nil {
		return err
	}

	//If everything is ok, return nil for the error
	return nil
}

func (vl *Voter) GetVoter(id int) (VoterItem, error) {

	// Check if item exists before trying to get it
	// this is a good practice, return an error if the
	// item does not exist
	var voterItem VoterItem
	pattern := redisKeyFromId(id)
	err := vl.getVoterFromRedis(pattern, &voterItem)
	if err != nil {
		return VoterItem{}, err
	}

	return voterItem, nil
}

// GetAllItems returns all items from the DB.  If successful it
// returns a slice of all of the items to the caller
// Preconditions:   (1) The database file must exist and be a valid
//
// Postconditions:
//
//	 (1) All items will be returned, if any exist
//		(2) If there is an error, it will be returned
//			along with an empty slice
//		(3) The database file will not be modified
func (vl *Voter) GetAllVoters() ([]VoterItem, error) {

	//Now that we have the DB loaded, lets crate a slice
	var voterList []VoterItem
	var voterItem VoterItem

	//Lets query redis for all of the items
	pattern := RedisKeyPrefix + "*"
	ks, _ := vl.client.Keys(vl.context, pattern).Result()
	for _, key := range ks {
		err := vl.getVoterFromRedis(key, &voterItem)
		if err != nil {
			return nil, err
		}
		voterList = append(voterList, voterItem)
	}

	return voterList, nil
}

// GetVoterPolls retrieves the voting history for a specific voter.
// It takes voter ID as input and returns their voting history as a slice of VoterHistory.
func (vl *Voter) GetVoterPolls(voterID int) ([]VoterHistory, error) {
	voterItem, err := vl.GetVoter(voterID)
	if err != nil {
		return nil, err
	}

	return voterItem.VoteHistory, nil
}

// GetVoterPoll retrieves a specific voting record for a voter.
// It takes voter ID and poll ID as input and returns the corresponding VoterHistory if found.
func (vl *Voter) GetVoterPoll(voterID, pollID int) (VoterHistory, error) {
	voterItem, err := vl.GetVoter(voterID)
	if err != nil {
		return VoterHistory{}, err
	}

	for _, history := range voterItem.VoteHistory {
		if history.PollId == pollID {
			return history, nil
		}
	}

	return VoterHistory{}, errors.New("poll not found for this voter")
}

// AddVoterPoll adds a new voting record for a voter.
func (vl *Voter) AddVoterPoll(voterPoll VoterHistory, voterId int) error {
	voterItem, err := vl.GetVoter(voterId)
	if err != nil {
		return err
	}

	for _, vh := range voterItem.VoteHistory {
		if vh.PollId == voterPoll.PollId {
			return errors.New("poll already exists")
		}
	}

	voterItem.VoteHistory = append(voterItem.VoteHistory, voterPoll)

	err = vl.UpdateVoter(voterItem)
	if err != nil {
		return err
	}

	return nil
}

// UpdateVoterPoll updates a voting record for a voter.
func (vl *Voter) UpdateVoterPoll(voterPoll VoterHistory, voterId int, pollId int) error {
	voterItem, err := vl.GetVoter(voterId)
	if err != nil {
		return err
	}

	for i, vh := range voterItem.VoteHistory {
		if vh.PollId == pollId {
			voterItem.VoteHistory[i] = voterPoll
			if err := vl.UpdateVoter(voterItem); err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("poll not found for this voter")
}

// DeleteVoterPoll deletes a voting record for a voter.
func (vl *Voter) DeleteVoterPoll(voterID, pollID int) error {
	voterItem, err := vl.GetVoter(voterID)
	if err != nil {
		return err
	}

	for i, history := range voterItem.VoteHistory {
		if history.PollId == pollID {
			voterItem.VoteHistory = append(voterItem.VoteHistory[:i], voterItem.VoteHistory[i+1:]...)
			err := vl.UpdateVoter(voterItem)
			if err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("poll not found for this voter")
}

// PrintItem accepts a ToDoItem and prints it to the console
// in a JSON pretty format. As some help, look at the
// json.MarshalIndent() function from our in class go tutorial.
func (vl *Voter) PrintVoter(voterItem VoterItem) {
	jsonBytes, _ := json.MarshalIndent(voterItem, "", "  ")
	fmt.Println(string(jsonBytes))
}

// PrintAllItems accepts a slice of ToDoItems and prints them to the console
// in a JSON pretty format.  It should call PrintItem() to print each item
// versus repeating the code.
func (vl *Voter) PrintAllVoter(voterList []VoterItem) {
	for _, voterItem := range voterList {
		vl.PrintVoter(voterItem)
	}
}

// JsonToItem accepts a json string and returns a ToDoItem
// This is helpful because the CLI accepts todo items for insertion
// and updates in JSON format.  We need to convert it to a ToDoItem
// struct to perform any operations on it.
func (t *Voter) JsonToVoter(jsonString string) (VoterItem, error) {
	var voterItem VoterItem
	err := json.Unmarshal([]byte(jsonString), &voterItem)
	if err != nil {
		return VoterItem{}, err
	}

	return voterItem, nil
}
