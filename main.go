package main

import ("database/sql"
		"time"
		"github.com/google/uuid"
	    "log"
	    "net/http"
	    "os"
	    "sync/atomic"
	    "github.com/joho/godotenv"
	    _ "github.com/lib/pq"
	    "fmt"
	    "encoding/json"
	    "strings"
	    "errors"
	    "sort"

	    "github.com/danarrigo/Chirpy/internal/database"
	    "github.com/danarrigo/Chirpy/internal/auth"
		)

type apiConfig struct {
	fileserverHits atomic.Int32
	db *database.Queries
	platform string 
	secret string
	polka string
}


func main(){
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platformString := os.Getenv("PLATFORM")
	secretString := os.Getenv("SECRET")
	polkaString := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)
	if err!=nil{
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiCfg := &apiConfig{db:dbQueries,platform:platformString,secret:secretString,polka:polkaString}
	serveMux := http.NewServeMux()
	server := &http.Server{
		Handler:serveMux,
		Addr : ":8080",
	}
	serveMux.Handle("/app/",apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("./")))))
	serveMux.HandleFunc("GET /api/healthz",handlerReadiness)
	serveMux.HandleFunc("GET /admin/metrics",apiCfg.handlerMetrics)
	serveMux.HandleFunc("POST /admin/reset",apiCfg.handlerReset)
	serveMux.HandleFunc("POST /api/users",apiCfg.handlerUsersCreate)
	serveMux.HandleFunc("POST /api/chirps",apiCfg.handlerCrudOps)
	serveMux.HandleFunc("GET /api/chirps",apiCfg.handlerChirpGetter)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}",apiCfg.handlerSpecificChirpGetter)
	serveMux.HandleFunc("POST /api/login",apiCfg.handlerUsersLogin)
	serveMux.HandleFunc("POST /api/refresh",apiCfg.handlerTokenRefresh)
	serveMux.HandleFunc("POST /api/revoke",apiCfg.handlerTokenRevoke)
	serveMux.HandleFunc("PUT /api/users",apiCfg.handlerChirpUpdater)
	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}",apiCfg.handlerChirpDeleter)
	serveMux.HandleFunc("POST /api/polka/webhooks",apiCfg.handlerWebhooks)
	err=server.ListenAndServe()
	if err!=nil{
		print(err)
	}
	
	
}
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter,r *http.Request){
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w,r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w,`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`,cfg.fileserverHits.Load())
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter,r *http.Request){
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}
	err := cfg.db.Reset(r.Context())
	if err != nil {
	    respondWithError(w, http.StatusInternalServerError, "Couldn't reset database")
	    return
	}
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func validateChirp(body string)(string,error){
	if len(body) > 140 {
			return "", errors.New("Chirp is too long")
		}
	badWords:=[]string{"kerfuffle", "sharbert", "fornax"}
	cleanWords := make([]string,0)
		for _,word:=range strings.Split(body," "){
			wordRn :=word
			for _,badWord := range badWords{
				if strings.ToLower(word)==strings.ToLower(badWord){
					wordRn="****"
					break
				}
			}
			cleanWords = append(cleanWords,wordRn)
		}
		return strings.Join(cleanWords, " "), nil
}


func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorRes struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorRes{
		Error: msg,
	})
}

func (cfg *apiConfig) handlerUsersCreate(w http.ResponseWriter,r *http.Request){
	type Email struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}
	request:=Email{}
	decoder:=json.NewDecoder(r.Body)
	if err:=decoder.Decode(&request);err!=nil{
		respondWithError(w,400,"Error Decoding")
		return
	}
	hashedPassword,err:=auth.HashPassword(request.Password)
	if err!=nil{
			respondWithError(w,http.StatusInternalServerError,"Error Creating User")
			return
	}
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email: request.Email,
		HashedPassword:hashedPassword,
	})
	if err!=nil{
		respondWithError(w,http.StatusInternalServerError,"Error Creating User")
		return
	}
	type User struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
		IsChirpyRed bool `json:"is_chirpy_red"`
	}
	respondWithJSON(w, http.StatusCreated, User{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func (cfg *apiConfig) handlerCrudOps(w http.ResponseWriter,r *http.Request){
	header:=r.Header;
	token,err:=auth.GetBearerToken(header);
	if err!=nil{
		respondWithError(w,401,"Error generating token")
		return
	}
	id,err:=auth.ValidateJWT(token,cfg.secret);
	if err!=nil{
			respondWithError(w,401,"Unauthorized")
			return
	}
	type Payload struct{
		Body string `json:"body"`
	}
	decoder:=json.NewDecoder(r.Body)
	respData:=Payload{}
	if err:=decoder.Decode(&respData);err!=nil{
		respondWithError(w,400,"error while decoding")
		return
	}
	cleaned, err := validateChirp(respData.Body)
	if err != nil {
	    respondWithError(w, http.StatusBadRequest, err.Error())
	    return
	}
	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
	    Body:   cleaned,
	    UserID: id,
	})
	if err != nil {
		    respondWithError(w, http.StatusBadRequest, err.Error())
		    return
	}
	type Chirp struct {
	    ID        uuid.UUID `json:"id"`
	    CreatedAt time.Time `json:"created_at"`
	    UpdatedAt time.Time `json:"updated_at"`
	    Body      string    `json:"body"`
	    UserID    uuid.UUID `json:"user_id"`
	}
	respondWithJSON(w, http.StatusCreated, Chirp{
	    ID:        chirp.ID,
	    CreatedAt: chirp.CreatedAt,
	    UpdatedAt: chirp.UpdatedAt,
	    Body:      chirp.Body,
	    UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig)handlerChirpGetter(w http.ResponseWriter,r *http.Request){
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	
	sorter := r.URL.Query().Get("sort")
	sort_dir := "desc"
	if sorter != "desc" {
		sort_dir="asc"
	}
	s := r.URL.Query().Get("author_id")
	if s!=""{
		parsedS,err:=uuid.Parse(s);
		if err!=nil{
			respondWithError(w,500,"error while getting chirps")
			return
		}
		chirps,err:=cfg.db.GetChirpsByAuthor(r.Context(),parsedS)
		if err!=nil{
				respondWithError(w,500,"error while getting chirps")
				return
		}
		results := []Chirp{}
		for _, dbChirp := range chirps {
			    results = append(results, Chirp{
			        ID:dbChirp.ID,
			        CreatedAt:dbChirp.CreatedAt,
			        UpdatedAt:dbChirp.UpdatedAt,
			        Body:dbChirp.Body,
			        UserID:dbChirp.UserID,
			    })
			}
		sort.Slice(results, func(i, j int) bool {
		    if sort_dir == "desc" {
		        return results[i].CreatedAt.After(results[j].CreatedAt)
		    }
		    return results[i].CreatedAt.Before(results[j].CreatedAt)
		})
		respondWithJSON(w, http.StatusOK, results)
		return
	}
	chirps,err:=cfg.db.GetChirps(r.Context())
	if err!=nil{
		respondWithError(w,500,"error while getting chirps")
		return
	}
	results := []Chirp{}
	for _, dbChirp := range chirps {
	    results = append(results, Chirp{
	        ID:dbChirp.ID,
	        CreatedAt:dbChirp.CreatedAt,
	        UpdatedAt:dbChirp.UpdatedAt,
	        Body:dbChirp.Body,
	        UserID:dbChirp.UserID,
	    })
	}
	sort.Slice(results, func(i, j int) bool {
	    if sort_dir == "desc" {
	        return results[i].CreatedAt.After(results[j].CreatedAt)
	    }
	    return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	respondWithJSON(w, http.StatusOK, results)
}

func (cfg *apiConfig)handlerSpecificChirpGetter(w http.ResponseWriter,r *http.Request){
	path:=r.PathValue("chirpID")
	parsedPath,err:=uuid.Parse(path)
	if err!=nil{
		respondWithError(w,400,"error while parsing")
		return
	}
	chirp,err:=cfg.db.GetChirp(r.Context(),parsedPath)
	if err!=nil{
		respondWithError(w,404,"error while querying")
		return
	}
	type Chirp struct {
			    ID        uuid.UUID `json:"id"`
			    CreatedAt time.Time `json:"created_at"`
			    UpdatedAt time.Time `json:"updated_at"`
			    Body      string    `json:"body"`
			    UserID    uuid.UUID `json:"user_id"`
	}	
	respondWithJSON(w, http.StatusOK, Chirp{
	    ID:        chirp.ID,
	    CreatedAt: chirp.CreatedAt,
	    UpdatedAt: chirp.UpdatedAt,
	    Body:      chirp.Body,
	    UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig)handlerUsersLogin(w http.ResponseWriter,r *http.Request){
	type Body struct{
		Password string `json:"password"`;
		Email string `json:"email"` ;
	}
	data:=Body{}
	decoder:=json.NewDecoder(r.Body)
	if err:=decoder.Decode(&data); err!=nil{
		respondWithError(w,400,"error while parsing")
		return
	}
	expiresInSeconds := 3600;
	user, err := cfg.db.GetUserByEmail(r.Context(), data.Email)
	if err != nil {
	    respondWithError(w, 401, "Incorrect email or password")
	    return
	}
	booel,err:=auth.CheckPasswordHash(data.Password,user.HashedPassword)
	if err!=nil{
		respondWithError(w, 401, "error")
		return
	}
	if booel ==false{
		respondWithError(w, 401, "Incorrect email or password")
		return
	}else{
		type response struct {
		    ID        uuid.UUID `json:"id"`
		    CreatedAt time.Time `json:"created_at"`
		    UpdatedAt time.Time `json:"updated_at"`
		    Email     string    `json:"email"`
		    Token	  string 	`json:"token"`
		    RefreshToken string `json:"refresh_token"`
		    IsChirpyRed bool `json:"is_chirpy_red"`
		}
		token, err := auth.MakeJWT(user.ID, cfg.secret, time.Duration(expiresInSeconds)*time.Second)
		if err != nil {
		    respondWithError(w, 500, "Error creating JWT")
		    return
		}
		refreshToken:=auth.MakeRefreshToken();
		err=cfg.db.InsertRefreshToken(r.Context(),database.InsertRefreshTokenParams{
			Token:refreshToken,
			CreatedAt:time.Now(),
			UpdatedAt:time.Now(),
			UserID:user.ID,	
			ExpiresAt: time.Now().Add(1440 * time.Hour),
		})
		if err!=nil{
			respondWithError(w, 500, "Error adding data to the database");
			return;
		}
		respondWithJSON(w,200,response{
			ID:user.ID,
			CreatedAt:user.CreatedAt,
			UpdatedAt:user.UpdatedAt,
			Email:user.Email,
			Token:token,
			RefreshToken:refreshToken,
			IsChirpyRed:user.IsChirpyRed,
		})
	}
	
}

func (cfg *apiConfig)handlerTokenRefresh(w http.ResponseWriter,r *http.Request){
	header:=r.Header;
	refreshToken,err:=auth.GetBearerToken(header);
	if err!=nil{
		respondWithError(w,400,"Error generating token")
		return
	}
	user,err:=cfg.db.GetUserFromRefreshToken(r.Context(),refreshToken);
	if err!=nil{
		respondWithError(w, 401, "Error finding user with specific token");
		return;
	}
	jwtToken,err:=auth.MakeJWT(user.ID, cfg.secret, time.Hour)
	if err!=nil{
		respondWithError(w,400,"Error generating token")
		return
	}
	type tokenStruct struct {
		Token string `json:"token"`
	}
	respondWithJSON(w,200,tokenStruct{Token:jwtToken})
	
}

func (cfg *apiConfig)handlerTokenRevoke(w http.ResponseWriter,r *http.Request){
	header:=r.Header;
	token,err:=auth.GetBearerToken(header);
	if err!=nil{
		respondWithError(w,400,"Error getting token")
		return
	}
	_,err=cfg.db.RevokeRefreshToken(r.Context(),token)
	if err!=nil{
		respondWithError(w,500,"Token not valid")
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return

}

func (cfg *apiConfig)handlerChirpUpdater(w http.ResponseWriter,r *http.Request){
	header:=r.Header;
	token,err:=auth.GetBearerToken(header);
	if err!=nil{
		respondWithError(w,401,"Error getting token")
		return
	}
	id,err:=auth.ValidateJWT(token,cfg.secret);
	if err!=nil{
			respondWithError(w,401,"Error validating token")
			return
	}
	type requestStruct struct {
			Password string `json:"password"`
			Email string `json:"email"`
		}
	data:=requestStruct{};
	decoder:=json.NewDecoder(r.Body)
	if err=decoder.Decode(&data);err!=nil{
		respondWithError(w,400,"Error decoding body")
		return
	}
	hashedPassword,err:=auth.HashPassword(data.Password);
	if err!=nil{
			respondWithError(w,400,"Error hashing password")
			return
	}
	user,err:=cfg.db.UpdateUser(r.Context(),database.UpdateUserParams{
		ID:id,
		HashedPassword:hashedPassword,
		Email:data.Email,
	})
	if err!=nil{
		respondWithError(w,401,"Error updating db")
		return 
	}
	respondWithJSON(w, http.StatusOK, struct {
	    ID           uuid.UUID    `json:"id"`
	    CreatedAt    time.Time `json:"created_at"`
	    UpdatedAt    time.Time `json:"updated_at"`
	    Email        string    `json:"email"`
	    IsChirpyRed bool `json:"is_chirpy_red"`
	}{
	    ID:           user.ID,
	    CreatedAt:    user.CreatedAt,
	    UpdatedAt:    user.UpdatedAt,
	    Email:        user.Email,
	    IsChirpyRed: user.IsChirpyRed,
	})
}

func (cfg *apiConfig)handlerChirpDeleter(w http.ResponseWriter,r *http.Request){
	header:=r.Header;
	token,err:=auth.GetBearerToken(header);
	if err!=nil{
		respondWithError(w,401,"Error getting token")
		return
	}
	id,err:=auth.ValidateJWT(token,cfg.secret);
	if err!=nil{
		respondWithError(w,401,"Error validating token")
		return
	}
	chirpIDStr := r.PathValue("chirpID")
	chirpID,err := uuid.Parse(chirpIDStr)
	if err!=nil{
		respondWithError(w,403,"Error Parsing");
		return;
	}
	chirp,err:=cfg.db.GetChirp(r.Context(),chirpID);
	if err!=nil{
		respondWithError(w,404,"Chirp not found")
		return
	}
	if chirp.UserID!=id{
		respondWithError(w,403,"Not authorized")
		return
	}
	err=cfg.db.DeleteChirp(r.Context(),chirpID);
	if err!=nil{
		respondWithError(w,400,"Error deleting")
		return
	}
	w.WriteHeader(204);
}

func (cfg *apiConfig)handlerWebhooks(w http.ResponseWriter,r *http.Request){
	authorizationKey,err:=auth.GetAPIKey(r.Header)
	if authorizationKey !=cfg.polka{
		respondWithError(w,401,"Unauthorized")
		return
	}
	type Request struct  {
		Event string `json:"event"`
		 Data struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}
	body := Request {}
	decoder := json.NewDecoder(r.Body);
	if err=decoder.Decode(&body);err!=nil{
		respondWithError(w,401,"Error decoding")
		return
	}
	if body.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}
	parsedID, err := uuid.Parse(body.Data.UserID)
	if err != nil {
	    respondWithError(w,401,"Error parsing")
	    return
	}
	_,err = cfg.db.UpgradeChirpy(r.Context(),parsedID);
	if err!= nil {
		respondWithError(w,404,"ID is not found");
		return
	}
	w.WriteHeader(204)
}
