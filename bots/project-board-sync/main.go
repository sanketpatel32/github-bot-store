// project-board-sync — automatically adds new issues and PRs to a GitHub
// Projects v2 board, and moves items between columns as their state changes
// (open -> In Progress when assigned, closed -> Done). Keeps boards honest
// without anyone dragging cards.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type issueEvent struct {
	Action string `json:"action"`
	Issue  *struct {
		Number   int    `json:"number"`
		NodeID   string `json:"node_id"`
		State    string `json:"state"`
		Assignee *struct {
			Login string `json:"login"`
		} `json:"assignee"`
	} `json:"issue"`
	PR *struct {
		Number   int    `json:"number"`
		NodeID   string `json:"node_id"`
		State    string `json:"state"`
		Merged   bool   `json:"merged"`
		Assignee *struct {
			Login string `json:"login"`
		} `json:"assignee"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
		Owner    struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"owner"`
	} `json:"repository"`
}

// findProject locates the project v2 by owner+number and returns its ID and
// the Status field with options. A project lives under either a user or an
// organization, and GitHub answers the wrong owner type with a NOT_FOUND
// error instead of an empty result — so the two scopes can't share one
// query. preferOrg picks which is tried first.
func findProject(client *ghbot.Client, owner string, number int, preferOrg bool) (projectID string, statusFieldID string, options map[string]string, err error) {
	scopes := []string{"user", "organization"}
	if preferOrg {
		scopes = []string{"organization", "user"}
	}
	var lookupErr error
	for _, scope := range scopes {
		query := fmt.Sprintf(`query($owner:String!, $number:Int!) {
  %s(login:$owner) { projectV2(number:$number) { id } }
}`, scope)
		data, err := client.GraphQL(query, map[string]any{"owner": owner, "number": number})
		if err != nil {
			if lookupErr == nil {
				lookupErr = err
			}
			continue
		}
		var result map[string]struct {
			ProjectV2 struct {
				ID string `json:"id"`
			} `json:"projectV2"`
		}
		if json.Unmarshal(data, &result) != nil {
			return "", "", nil, fmt.Errorf("parse project lookup")
		}
		if projectID = result[scope].ProjectV2.ID; projectID != "" {
			break
		}
	}
	if projectID == "" {
		if lookupErr != nil {
			return "", "", nil, lookupErr
		}
		return "", "", nil, fmt.Errorf("project %d not found for %s", number, owner)
	}

	// fetch status field options
	fieldsQuery := `query($project:ID!) {
  node(id:$project) {
    ... on ProjectV2 {
      fields(first:20) {
        nodes {
          ... on ProjectV2SingleSelectField {
            id
            name
            options { id name }
          }
        }
      }
    }
  }
}`
	fdata, err := client.GraphQL(fieldsQuery, map[string]any{"project": projectID})
	if err != nil {
		return projectID, "", nil, nil // status sync optional — degrade gracefully
	}
	var fresult struct {
		Node struct {
			Fields struct {
				Nodes []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Options []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"options"`
				} `json:"nodes"`
			} `json:"fields"`
		} `json:"node"`
	}
	if json.Unmarshal(fdata, &fresult) == nil {
		for _, f := range fresult.Node.Fields.Nodes {
			if strings.EqualFold(f.Name, "Status") {
				statusFieldID = f.ID
				options = map[string]string{}
				for _, o := range f.Options {
					options[strings.ToLower(o.Name)] = o.ID
				}
			}
		}
	}
	return projectID, statusFieldID, options, nil
}

func addToProject(client *ghbot.Client, projectID, nodeID string) (itemID string, err error) {
	mutation := `mutation($project:ID!, $content:ID!) {
  addProjectV2ItemById(input:{projectId:$project, contentId:$content}) {
    item { id }
  }
}`
	data, err := client.GraphQL(mutation, map[string]any{"project": projectID, "content": nodeID})
	if err != nil {
		return "", err
	}
	var result struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if json.Unmarshal(data, &result) != nil {
		return "", fmt.Errorf("parse add result")
	}
	return result.AddProjectV2ItemById.Item.ID, nil
}

func setStatus(client *ghbot.Client, projectID, itemID, fieldID, optionID string) {
	mutation := `mutation($project:ID!, $item:ID!, $field:ID!, $value:String!) {
  updateProjectV2ItemFieldValue(input:{
    projectId:$project, itemId:$item, fieldId:$field,
    value:{singleSelectOptionId:$value}
  }) { projectV2Item { id } }
}`
	_, err := client.GraphQL(mutation, map[string]any{
		"project": projectID, "item": itemID, "field": fieldID, "value": optionID,
	})
	if err != nil {
		log.Printf("set status: %v", err)
	}
}

func main() {
	cfg := ghbot.LoadConfig("project-board-sync", "8132")
	if ghbot.HandleCLIArgs("project-board-sync", cfg, "webhook") {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	projectNum := 0
	fmt.Sscanf(os.Getenv("PROJECT_NUMBER"), "%d", &projectNum)
	owner := os.Getenv("PROJECT_OWNER") // user or org login owning the project

	ghbot.Serve(cfg, "project-board-sync", func(event string, payload []byte) {
		if projectNum == 0 || owner == "" {
			log.Printf("PROJECT_NUMBER/PROJECT_OWNER not set — idle")
			return
		}
		if event != "issues" && event != "pull_request" {
			return
		}
		var ev issueEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}

		var nodeID, kind, state string
		var number int
		assigned := false
		merged := false
		if ev.Issue != nil {
			nodeID, kind, state, number = ev.Issue.NodeID, "issue", ev.Issue.State, ev.Issue.Number
			assigned = ev.Issue.Assignee != nil
		} else if ev.PR != nil {
			nodeID, kind, state, number = ev.PR.NodeID, "PR", ev.PR.State, ev.PR.Number
			merged = ev.PR.Merged
			assigned = ev.PR.Assignee != nil // assigned PRs are started work too (#109)
		} else {
			return
		}
		if nodeID == "" {
			return
		}

		switch ev.Action {
		case "opened", "reopened", "assigned", "unassigned", "closed":
		default:
			return
		}

		// the project usually lives on the repo's owner account — try that
		// scope first so the common case costs one query and no NOT_FOUND
		preferOrg := ev.Repo.Owner.Type == "Organization" || !strings.EqualFold(ev.Repo.Owner.Login, owner)
		projectID, statusFieldID, options, err := findProject(client, owner, projectNum, preferOrg)
		if err != nil {
			log.Printf("project lookup: %v", err)
			return
		}
		itemID, err := addToProject(client, projectID, nodeID)
		if err != nil {
			log.Printf("add %s #%d: %v", kind, number, err)
			return
		}
		log.Printf("added %s #%d to project", kind, number)

		// best-effort status placement
		if statusFieldID == "" || len(options) == 0 {
			return
		}
		var want string
		switch {
		case state == "closed" && merged:
			want = "done"
		case state == "closed":
			want = "done"
		case assigned:
			want = "in progress"
		default:
			want = "todo"
		}
		// find closest matching option name
		for name, id := range options {
			if strings.Contains(name, want) || (want == "todo" && strings.Contains(name, "to do")) {
				setStatus(client, projectID, itemID, statusFieldID, id)
				log.Printf("%s #%d -> status %q", kind, number, name)
				return
			}
		}
	})
}
