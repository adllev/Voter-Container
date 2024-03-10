package api

import (
	"log"
	"net/http"

	"github.com/adllev/Voter-Container/voter-api/db"
	"github.com/gofiber/fiber/v2"
)

// The api package creates and maintains a reference to the data handler
// this is a good design practice
type VoterAPI struct {
	db *db.VoterList
}

func New() (*VoterAPI, error) {
	dbHandler, err := db.NewVoterList()
	if err != nil {
		return nil, err
	}

	return &VoterAPI{db: dbHandler}, nil
}

//Below we implement the API functions.  Some of the framework
//things you will see include:
//   1) How to extract a parameter from the URL, for example
//	  the id parameter in /todo/:id
//   2) How to extract the body of a POST request
//   3) How to return JSON and a correctly formed HTTP status code
//	  for example, 200 for OK, 404 for not found, etc.  This is done
//	  using the c.JSON() function
//   4) How to return an error code and abort the request.  This is
//	  done using the c.AbortWithStatus() function

// implementation for GET /todo
// returns all todos
func (td *VoterAPI) ListAllVoters(c *fiber.Ctx) error {

	voterList, err := td.db.GetAllVoters()
	if err != nil {
		log.Println("Error Getting All Voters: ", err)
		return fiber.NewError(http.StatusNotFound,
			"Error Getting All Voters")
	}
	//Note that the database returns a nil slice if there are no items
	//in the database.  We need to convert this to an empty slice
	//so that the JSON marshalling works correctly.  We want to return
	//an empty slice, not a nil slice. This will result in the json being []
	if voterList == nil {
		voterList = make([]db.Voter, 0)
	}

	return c.JSON(voterList)
}

// implementation for GET /todo/:id
// returns a single todo
func (td *VoterAPI) GetVoter(c *fiber.Ctx) error {

	//Note go is minimalistic, so we have to get the
	//id parameter using the Param() function, and then
	//convert it to an int64 using the strconv package
	id, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	//Note that ParseInt always returns an int64, so we have to
	//convert it to an int before we can use it.
	voter, err := td.db.GetVoter(id)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	//Git will automatically convert the struct to JSON
	//and set the content-type header to application/json
	return c.JSON(voter)
}

// implementation for POST /todo
// adds a new todo
func (td *VoterAPI) PostVoter(c *fiber.Ctx) error {
	var voter db.Voter

	//With HTTP based APIs, a POST request will usually
	//have a body that contains the data to be added
	//to the database.  The body is usually JSON, so
	//we need to bind the JSON to a struct that we
	//can use in our code.
	//This framework exposes the raw body via c.Request.Body
	//but it also provides a helper function BodyParser
	//that will extract the body, convert it to JSON and
	//bind it to a struct for us.  It will also report an error
	//if the body is not JSON or if the JSON does not match
	//the struct we are binding to.
	if err := c.BodyParser(&voter); err != nil {
		log.Println("Error binding JSON: ", err)
		return fiber.NewError(http.StatusBadRequest)
	}

	if err := td.db.AddVoter(voter); err != nil {
		log.Println("Error adding item: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.JSON(voter)
}

// implementation for PUT /todo
// Web api standards use PUT for Updates
func (td *VoterAPI) UpdateVoter(c *fiber.Ctx) error {
	var voter db.Voter
	if err := c.BodyParser(&voter); err != nil {
		log.Println("Error binding JSON: ", err)
		return fiber.NewError(http.StatusBadRequest)
	}

	if err := td.db.UpdateVoter(voter); err != nil {
		log.Println("Error updating voter: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.JSON(voter)
}

// implementation for DELETE /todo/:id
// deletes a todo
func (td *VoterAPI) DeleteVoter(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	if err := td.db.DeleteVoter(id); err != nil {
		log.Println("Error deleting voter: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).SendString("Delete OK")
}

// implementation for DELETE /todo
// deletes all todos
func (td *VoterAPI) DeleteAllVoters(c *fiber.Ctx) error {

	if err := td.db.DeleteAll(); err != nil {
		log.Println("Error deleting all items: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.Status(http.StatusOK).SendString("Delete All OK")
}

// implementation for GET /voters/:id/polls
func (td *VoterAPI) GetVoterPolls(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	voter, err := td.db.GetVoter(id)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	return c.JSON(voter.VoteHistory)
}

// implementation for GET /voters/:id/polls/:pollid
func (td *VoterAPI) GetVoterPoll(c *fiber.Ctx) error {
	voterID, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	pollID, err := c.ParamsInt("pollid")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	voter, err := td.db.GetVoter(voterID)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	for _, history := range voter.VoteHistory {
		if history.PollId == pollID {
			return c.JSON(history)
		}
	}

	return fiber.NewError(http.StatusNotFound)
}

// implementation for POST /voters/:id/polls/:pollid
func (td *VoterAPI) PostVoterPoll(c *fiber.Ctx) error {
	voterID, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	pollID, err := c.ParamsInt("pollid")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	var voterHistory db.VoterHistory
	if err := c.BodyParser(&voterHistory); err != nil {
		log.Println("Error binding JSON: ", err)
		return fiber.NewError(http.StatusBadRequest)
	}

	voter, err := td.db.GetVoter(voterID)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	voterHistory.PollId = pollID
	voter.VoteHistory = append(voter.VoteHistory, voterHistory)

	if err := td.db.UpdateVoter(voter); err != nil {
		log.Println("Error updating voter: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.JSON(voterHistory)
}

// implementation for PUT /voters/:id/polls/:pollid
func (td *VoterAPI) UpdateVoterPoll(c *fiber.Ctx) error {
	voterID, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	pollID, err := c.ParamsInt("pollid")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	var updatedHistory db.VoterHistory
	if err := c.BodyParser(&updatedHistory); err != nil {
		log.Println("Error binding JSON: ", err)
		return fiber.NewError(http.StatusBadRequest)
	}

	voter, err := td.db.GetVoter(voterID)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	// Find the index of the history with the given poll ID
	var index = -1
	for i, history := range voter.VoteHistory {
		if history.PollId == pollID {
			index = i
			break
		}
	}

	if index == -1 {
		return fiber.NewError(http.StatusNotFound, "Poll not found for the voter")
	}

	// Update the VoterHistory slice
	voter.VoteHistory[index] = updatedHistory

	if err := td.db.UpdateVoter(voter); err != nil {
		log.Println("Error updating voter: ", err)
		return fiber.NewError(http.StatusInternalServerError)
	}

	return c.JSON(updatedHistory)
}

// implementation for DELETE /voters/:id/polls/:pollid
func (td *VoterAPI) DeleteVoterPoll(c *fiber.Ctx) error {
	voterID, err := c.ParamsInt("id")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	pollID, err := c.ParamsInt("pollid")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest)
	}

	voter, err := td.db.GetVoter(voterID)
	if err != nil {
		log.Println("Voter not found: ", err)
		return fiber.NewError(http.StatusNotFound)
	}

	for i, history := range voter.VoteHistory {
		if history.PollId == pollID {
			voter.VoteHistory = append(voter.VoteHistory[:i], voter.VoteHistory[i+1:]...)
			if err := td.db.UpdateVoter(voter); err != nil {
				log.Println("Error updating voter: ", err)
				return fiber.NewError(http.StatusInternalServerError)
			}
			return c.Status(http.StatusOK).SendString("Delete OK")
		}
	}

	return fiber.NewError(http.StatusNotFound)
}

// implementation of GET /voters/health. It is a good practice to build in a
// health check for your API.  Below the results are just hard coded
// but in a real API you can provide detailed information about the
// health of your API with a Health Check
func (td *VoterAPI) HealthCheck(c *fiber.Ctx) error {
	return c.Status(http.StatusOK).
		JSON(fiber.Map{
			"status":             "ok",
			"version":            "1.0.0",
			"uptime":             100,
			"users_processed":    1000,
			"errors_encountered": 10,
		})
}
