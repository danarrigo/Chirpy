package auth

import ("time"
		"errors"
		"net/http"
		"strings"
		"crypto/rand"
		"encoding/hex"

		"github.com/alexedwards/argon2id"
		"github.com/golang-jwt/jwt/v5"
		"github.com/google/uuid"
		)
func HashPassword(password string) (string, error){
	return argon2id.CreateHash(password,argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error){
	return argon2id.ComparePasswordAndHash(password,hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
	now := time.Now().UTC()
	token:=jwt.NewWithClaims(jwt.SigningMethodHS256,jwt.RegisteredClaims{
		Issuer:"chirpy-access",
		IssuedAt:&jwt.NumericDate{Time:now},
		ExpiresAt:&jwt.NumericDate{Time:now.Add(expiresIn)},
		Subject:userID.String(),
	})
	str, err := token.SignedString([]byte(tokenSecret))
	if err!=nil{
		return "",err
	}
	return str,nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error){
	token,err:=jwt.ParseWithClaims(tokenString,&jwt.RegisteredClaims{},
	func(token *jwt.Token) (interface{}, error) {
	    return []byte(tokenSecret), nil
	})
	if err!=nil{
		return  uuid.UUID{},err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
	    return uuid.UUID{}, errors.New("invalid claims")
	}
	userID:=claims.Subject
	id,err:=uuid.Parse(userID)
	if err!=nil{
		return uuid.UUID{}, errors.New("failed parsing")
	}
	return id,nil
}

func GetBearerToken(headers http.Header)(string,error){
	value:=headers.Get("Authorization")
	if value==""{
		return "",errors.New("No Authorization Header Set")
	}
	split_value := strings.Split(value," ");
	if len(split_value)==2{
		if split_value[0]=="Bearer"{
			token_string:=split_value[1];
			return token_string,nil
		}
	}
	return "",errors.New("Error with Header Format")
}

func MakeRefreshToken() string {
	data:=make([]byte,32);
	_,err:=rand.Read(data);
	if err!=nil{
		return ""
	}
	refreshString:=hex.EncodeToString(data);
	return refreshString;
}

func GetAPIKey(headers http.Header) (string, error){
	value:=headers.Get("Authorization")
	if value==""{
			return "",errors.New("No Authorization Header Set")
	}
	split_value := strings.Split(value," ");
		if len(split_value)==2{
			if split_value[0]=="ApiKey"{
				apiString:=split_value[1];
				return apiString,nil
			}
		}
	return "",errors.New("Error with Header Format")
}
