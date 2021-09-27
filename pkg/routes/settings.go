package routes

import (
	"encoding/json"
	"github.com/go-chi/chi"
	"net/http"
)

func MakeSettingsRouter(sub chi.Router) {
	sub.Get("/", EdgeSettings)
}

func EdgeSettings(w http.ResponseWriter, r *http.Request) {
	type Form []struct {
		Fields []struct {
			Component string `json:"component"`
			Label     string `json:"label"`
			Name      string `json:"name"`
		} `json:"fields"`
	}

	//	component := `
	//	{
	//		"component": "checkbox",
	//		"label": "Checkbox",
	//		"name": "checkbox"
	//		}
	//		`

	json.NewEncoder(w).Encode()
}
