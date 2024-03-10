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

// // ToDoItem is the struct that represents a single ToDo item
// type ToDoItem struct {
// 	Id     int    `json:"id"`
// 	Title  string `json:"title"`
// 	IsDone bool   `json:"done"`
// }

const (
	RedisNilError        = "redis: nil"
	RedisDefaultLocation = "0.0.0.0:6379"
	RedisKeyPrefix       = "voters:"
)

type cache struct {
	cacheClient *redis.Client
	jsonHelper  *rejson.Handler
	context     context.Context
}

// VoterHistory is the struct that represents a single VoterHistory item
type VoterHistory struct {
	PollId   int
	VoteId   int
	VoteDate time.Time
}

// Voter is the struct that represents a single Voter item
type Voter struct {
	VoterId     int
	Name        string
	Email       string
	VoteHistory []VoterHistory
}

type VoterList struct {
	cache
}

// NewVoterList is a constructor function that returns a pointer to a new VoterList struct
// It uses the default Redis URL with NewVoterListWithCacheInstance.
func NewVoterList() (*VoterList, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = RedisDefaultLocation
	}
	log.Println("DEBUG: USING REDIS URL: ", redisURL)
	return NewVoterListWithCacheInstance(redisURL)
}

func NewVoterListWithCacheInstance(location string) (*VoterList, error) {
	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: location,
	})
	ctx := context.Background()

	// Check Redis connection
	err := client.Ping(ctx).Err()
	if err != nil {
		log.Println("Error connecting to Redis: ", err)
	}

	// Initialize JSON helper
	jsonHelper := rejson.NewReJSONHandler()
	jsonHelper.SetGoRedisClientWithContext(ctx, client)

	return &VoterList{
		cache: cache{
			cacheClient: client,
			jsonHelper:  jsonHelper,
			context:     ctx,
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

// Helper to return a ToDoItem from redis provided a key
func (vl *VoterList) getVoterFromRedis(voterID string, voter *Voter) error {

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
	err = json.Unmarshal(itemObject.([]byte), voter)
	if err != nil {
		return err
	}

	return nil
}

// AddVoter adds a new voter to the database
func (vl *VoterList) AddVoter(voter Voter) error {

	//Before we add an item to the DB, lets make sure
	//it does not exist, if it does, return an error
	redisKey := redisKeyFromId(voter.VoterId)
	var existingItem Voter
	if err := vl.getVoterFromRedis(redisKey, &existingItem); err == nil {
		return errors.New("voter already exists")
	}

	//Add item to database with JSON Set
	if _, err := vl.jsonHelper.JSONSet(redisKey, ".", voter); err != nil {
		return err
	}

	//If everything is ok, return nil for the error
	return nil
}

// DeleteVoter deletes a voter from the database
func (vl *VoterList) DeleteVoter(id int) error {

	pattern := redisKeyFromId(id)
	numDeleted, err := vl.cacheClient.Del(vl.context, pattern).Result()
	if err != nil {
		return err
	}
	if numDeleted == 0 {
		return errors.New("attempted to delete non-existent voterr")
	}

	return nil
}

// DeleteAll deletes all voters from the database
func (vl *VoterList) DeleteAll() error {

	pattern := RedisKeyPrefix + "*"
	ks, _ := vl.cacheClient.Keys(vl.context, pattern).Result()
	//Note delete can take a collection of keys.  In go we can
	//expand a slice into individual arguments by using the ...
	//operator
	numDeleted, err := vl.cacheClient.Del(vl.context, ks...).Result()
	if err != nil {
		return err
	}

	if numDeleted != int64(len(ks)) {
		return errors.New("one or more items could not be deleted")
	}

	return nil
}

// UpdateVoter updates a voter in the database
func (t *VoterList) UpdateVoter(item ToDoItem) error {

	//Before we add an item to the DB, lets make sure
	//it does not exist, if it does, return an error
	redisKey := redisKeyFromId(item.Id)
	var existingItem Voter
	if err := t.getVoterFromRedis(redisKey, &existingItem); err != nil {
		return errors.New("voter does not exist")
	}

	//Add item to database with JSON Set.  Note there is no update
	//functionality, so we just overwrite the existing item
	if _, err := t.jsonHelper.JSONSet(redisKey, ".", item); err != nil {
		return err
	}

	//If everything is ok, return nil for the error
	return nil
}

func (vl *VoterList) GetVoter(id int) (Voter, error) {

	// Check if item exists before trying to get it
	// this is a good practice, return an error if the
	// item does not exist
	var voter Voter
	pattern := redisKeyFromId(id)
	err := vl.getVoterFromRedis(pattern, &voter)
	if err != nil {
		return Voter{}, err
	}

	return voter, nil
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
func (vl *VoterList) GetAllVoters() ([]Voter, error) {

	//Now that we have the DB loaded, lets crate a slice
	var voterList []Voter
	var voter Voter

	//Lets query redis for all of the items
	pattern := RedisKeyPrefix + "*"
	ks, _ := vl.cacheClient.Keys(vl.context, pattern).Result()
	for _, key := range ks {
		err := vl.getVoterFromRedis(key, &voter)
		if err != nil {
			return nil, err
		}
		voterList = append(voterList, voter)
	}

	return voterList, nil
}

// PrintItem accepts a ToDoItem and prints it to the console
// in a JSON pretty format. As some help, look at the
// json.MarshalIndent() function from our in class go tutorial.
func (vl *VoterList) PrintVoter(voter Voter) {
	jsonBytes, _ := json.MarshalIndent(voter, "", "  ")
	fmt.Println(string(jsonBytes))
}

// PrintAllItems accepts a slice of ToDoItems and prints them to the console
// in a JSON pretty format.  It should call PrintItem() to print each item
// versus repeating the code.
func (vl *VoterList) PrintAllVoter(voterList []Voter) {
	for _, item := range voterList {
		vl.PrintVoter(item)
	}
}

// JsonToItem accepts a json string and returns a ToDoItem
// This is helpful because the CLI accepts todo items for insertion
// and updates in JSON format.  We need to convert it to a ToDoItem
// struct to perform any operations on it.
func (t *VoterList) JsonToItem(jsonString string) (Voter, error) {
	var voter Voter
	err := json.Unmarshal([]byte(jsonString), &voter)
	if err != nil {
		return Voter{}, err
	}

	return voter, nil
}
