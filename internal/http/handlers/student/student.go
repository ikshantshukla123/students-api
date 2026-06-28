package student

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/ikshantshukla123/students-api/internal/types"
	"github.com/ikshantshukla123/students-api/internal/utils/response"
	"github.com/go-playground/validator/v10"
)


func New() http.HandlerFunc{
	return func(w http.ResponseWriter,r *http.Request){
		slog.Info("creating a student")
		


		var student types.Student

	err := 	json.NewDecoder(r.Body).Decode(&student)
		if errors.Is(err,io.EOF){
			response.WriteJson(w,http.StatusBadRequest,response.GeneralError(err))
			return
		}
		if err != nil{
			response.WriteJson(w,http.StatusBadRequest,response.GeneralError(err))
			return
		}
		//request validation (we can do it manually but not recomended we will use build in package)

		if err := validator.New().Struct(student); err != nil {

			validateErrs := err.(validator.ValidationErrors)
			response.WriteJson(w,http.StatusBadRequest,response.ValidationError(validateErrs))
			return
		}


		response.WriteJson(w,http.StatusCreated,map[string]string{"success":"OK"})
	 }
}