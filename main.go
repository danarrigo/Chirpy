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

	    "github.com/danarrigo/Chirpy/internal/database"
	    "github.com/danarrigo/Chirpy/internal/auth"
		)

type apiConfig struct {
	fileserverHits atomic.Int32
	db *database.Queries
	platform string 
	secret string
}


func main(){
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platformString := os.Getenv("PLATFORM")
	secretString := os.Getenv("SECRET")
	db, err := sql.Open("postgres", dbURL)
	if err!=nil{
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiCfg := &apiConfig{db:dbQueries,platform:platformString,secret:secretString}
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
	}
	respondWithJSON(w, http.StatusCreated, User{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
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
	chirps,err:=cfg.db.GetChirps(r.Context())
	if err!=nil{
		respondWithError(w,500,"error while getting chirps")
		return
	}
	type Chirp struct {
		    ID        uuid.UUID `json:"id"`
		    CreatedAt time.Time `json:"created_at"`
		    UpdatedAt time.Time `json:"updated_at"`
		    Body      string    `json:"body"`
		    UserID    uuid.UUID `json:"user_id"`
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
		ExpiresInSeconds int `json:"expires_in_seconds"`;
	}
	data:=Body{}
	decoder:=json.NewDecoder(r.Body)
	if err:=decoder.Decode(&data); err!=nil{
		respondWithError(w,400,"error while parsing")
		return
	}
	if data.ExpiresInSeconds == 0 || data.ExpiresInSeconds > 3600 {
		data.ExpiresInSeconds = 3600;
	}
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
		}
		token, err := auth.MakeJWT(user.ID, cfg.secret, time.Duration(data.ExpiresInSeconds)*time.Second)
		if err != nil {
		    respondWithError(w, 500, "Error creating JWT")
		    return
		}
		respondWithJSON(w,200,response{
			ID:user.ID,
			CreatedAt:user.CreatedAt,
			UpdatedAt:user.UpdatedAt,
			Email:user.Email,
			Token:token,
		})
	}
	
}
