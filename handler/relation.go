package handler

import (
	"errors"
	log "github.com/Sirupsen/logrus"

	"github.com/mitchellh/mapstructure"

	"github.com/oursky/skygear/oderr"
	"github.com/oursky/skygear/router"
)

type relationPayload struct {
	Name      string   `json:"name"`
	Direction string   `json:"direction"`
	Target    []string `json:"targets"`
}

func relationColander(data map[string]interface{}, result *relationPayload) error {
	mapDecoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  result,
		TagName: "json",
	})
	if err := mapDecoder.Decode(data); err != nil {
		return oderr.NewRequestJSONInvalidErr(err)
	}
	if result.Name != "friend" && result.Name != "follow" {
		return oderr.NewRequestInvalidErr(
			errors.New("Only friend and follow relation is supported"))
	}
	if result.Direction != "" {
		if result.Direction != "active" && result.Direction != "passive" && result.Direction != "mutual" {
			return oderr.NewRequestInvalidErr(
				errors.New("Only active, passive and mutual direction is supported"))
		}
	}
	return nil
}

// RelationQueryHandler query user from current users' relation
// curl -X POST -H "Content-Type: application/json" \
//   -d @- http://localhost:3000/ <<EOF
// {
//     "action": "relation:query",
//     "access_token": "ACCESS_TOKEN",
//     "name": "follow",
//     "direction": "active"
// }
// EOF
func RelationQueryHandler(rpayload *router.Payload, response *router.Response) {
	log.Debug("RelationQueryHandler")
	payload := relationPayload{}
	if err := relationColander(rpayload.Data, &payload); err != nil {
		response.Err = err
		return
	}
	result := rpayload.DBConn.QueryRelation(
		rpayload.UserInfoID, payload.Name, payload.Direction)
	resultList := make([]interface{}, 0, len(result))
	for _, userinfo := range result {
		resultList = append(resultList, struct {
			ID   string      `json:"id"`
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		}{userinfo.ID, "user", userinfo})
	}
	response.Result = resultList
}

// RelationAddHandler add current user relation
// curl -X POST -H "Content-Type: application/json" \
//   -d @- http://localhost:3000/ <<EOF
// {
//     "action": "relation:add",
//     "access_token": "ACCESS_TOKEN",
//     "name": "follow",
//     "targets": [
//         "1001",
//         "1002"
//     ]
// }
// EOF
//
// {
//     "request_id": "REQUEST_ID",
//     "result": [
//         {
//             "id": "1001",
//         },
//         {
//             "id": "1002",
//             "type": "error",
//             "data": {
//                 "type": "ResourceFetchFailure",
//                 "code": 101,
//                 "message": "failed to fetch user id = 1002"
//             }
//         }
//     ]
// }
func RelationAddHandler(rpayload *router.Payload, response *router.Response) {
	log.Debug("RelationAddHandler")
	payload := relationPayload{}
	if err := relationColander(rpayload.Data, &payload); err != nil {
		response.Err = err
		return
	}
	results := make([]interface{}, 0, len(payload.Target))
	for s := range payload.Target {
		target := payload.Target[s]
		err := rpayload.DBConn.AddRelation(rpayload.UserInfoID, payload.Name, target)
		if err != nil {
			log.WithFields(log.Fields{
				"target": target,
				"err":    err,
			}).Debugln("failed to add relation")
			results = append(results, struct {
				ID   string      `json:"id"`
				Type string      `json:"type"`
				Data oderr.Error `json:"data"`
			}{target, "error", oderr.NewResourceFetchFailureErr("user", target)})
		} else {
			results = append(results, struct {
				ID string `json:"id"`
			}{target})
		}
	}
	response.Result = results
}

// RelationRemoveHandler remove a users' relation to other users
// curl -X POST -H "Content-Type: application/json" \
//   -d @- http://localhost:3000/ <<EOF
// {
//     "action": "relation:remove",
//     "access_token": "ACCESS_TOKEN",
//     "name": "follow",
//     "targets": [
//         "1001",
//         "1002"
//     ]
// }
// EOF
func RelationRemoveHandler(rpayload *router.Payload, response *router.Response) {
	log.Debug("RelationRemoveHandler")
	payload := relationPayload{}
	if err := relationColander(rpayload.Data, &payload); err != nil {
		response.Err = err
		return
	}
	results := make([]interface{}, 0, len(payload.Target))
	for s := range payload.Target {
		target := payload.Target[s]
		err := rpayload.DBConn.RemoveRelation(rpayload.UserInfoID, payload.Name, target)
		if err != nil {
			log.WithFields(log.Fields{
				"target": target,
				"err":    err,
			}).Debugln("failed to remmove user")
			results = append(results, struct {
				ID   string      `json:"id"`
				Type string      `json:"type"`
				Data interface{} `json:"data"`
			}{target, "error", err})
		} else {
			results = append(results, struct {
				ID string `json:"id"`
			}{target})
		}
	}
	response.Result = results
}
