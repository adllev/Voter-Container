package tests

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/adllev/Voter-Container/voter-api/db"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
)

var (
	BASE_API = "http://localhost:1080"

	cli = resty.New()
)

func TestMain(m *testing.M) {

	//SETUP GOES FIRST
	rsp, err := cli.R().Delete(BASE_API + "/voters")

	if rsp.StatusCode() != 200 {
		log.Printf("error clearing database, %v", err)
		os.Exit(1)
	}

	code := m.Run()

	//CLEANUP

	//Now Exit
	os.Exit(code)
}

func Test_AddSingleVoter(t *testing.T) {
	newVoterItem := db.VoterItem{
		VoterId:     1,
		Name:        "Jane Smith",
		Email:       "jane@example.com",
		VoteHistory: nil,
	}

	rsp, err := cli.R().
		SetBody(newVoterItem).
		SetResult(&newVoterItem).
		Post(BASE_API + "/voters")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())
}

func Test_AddSingleVoterPoll(t *testing.T) {
	newVoterPoll := db.VoterHistory{
		PollId:   1,
		VoteId:   1,
		VoteDate: time.Now(),
	}

	rsp, err := cli.R().
		SetBody(newVoterPoll).
		SetResult(&newVoterPoll).
		Post(BASE_API + "/voters/1/polls/1")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())

}

func Test_GetAllVoters(t *testing.T) {
	var items []db.VoterItem

	rsp, err := cli.R().SetResult(&items).Get(BASE_API + "/voters")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())

	assert.Equal(t, 1, len(items))
}

func Test_GetSingleVoter(t *testing.T) {
	var voterItem db.VoterItem

	rsp, err := cli.R().SetResult(&voterItem).Get(BASE_API + "/voters/1")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())

	assert.Equal(t, 1, voterItem.VoterId)
	assert.Equal(t, "Jane Smith", voterItem.Name)
	assert.Equal(t, "jane@example.com", voterItem.Email)
}

func Test_GetVoterPolls(t *testing.T) {
	var voterHistory []db.VoterHistory

	rsp, err := cli.R().SetResult(&voterHistory).Get(BASE_API + "/voters/1/polls")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())
}

func Test_GetSingleVoterPoll(t *testing.T) {
	var voterPoll db.VoterHistory

	rsp, err := cli.R().SetResult(&voterPoll).Get(BASE_API + "/voters/1/polls/1")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())

	assert.Equal(t, 1, voterPoll.PollId)
	assert.Equal(t, 1, voterPoll.VoteId)
}

func Test_GetVotersHealth(t *testing.T) {
	rsp, err := cli.R().Get(BASE_API + "/voters/health")

	assert.Nil(t, err)
	assert.Equal(t, 200, rsp.StatusCode())
}
