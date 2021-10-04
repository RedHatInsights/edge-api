package routes

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi"
	"io/ioutil"
	"net/http"
	"os"
)

func MakeSettingsRouter(sub chi.Router) {
	sub.Get("/", EdgeSettings)
}

type Forms []struct {
	Fields []struct {
		Component    string `json:"component"`
		Label        string `json:"label"`
		Name         string `json:"name"`
		IsRequired   bool   `json:"isRequired,omitempty"`
		Title        string `json:"title"`
		HelpText     string `json:"helpText"`
		InitialValue string `json:"initialValue"`
		Description  string `json:"description"`
		Validate     []struct {
			Type string `json:"type"`
		} `json:"validate,omitempty"`
	} `json:"fields"`
}

func EdgeSettings(w http.ResponseWriter, r *http.Request) {
	jsonFile, err := os.Open("./pkg/services/template_form/form.json")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	
	var forms Forms
	json.Unmarshal(byteValue, &forms)

	fmt.Println(forms)

	json.NewEncoder(w).Encode(forms)
}
