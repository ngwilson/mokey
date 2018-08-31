package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo"
	"github.com/ory/hydra/sdk/go/hydra/swagger"
	log "github.com/sirupsen/logrus"
	"github.com/ubccr/mokey/model"
)

func (h *Handler) ConsentGet(c echo.Context) error {
	// Get the challenge from the query.
	challenge := c.Request().URL.Query().Get("consent_challenge")
	if challenge == "" {
		log.WithFields(log.Fields{
			"ip": c.RealIP(),
		}).Error("Consent endpoint was called without a consent challenge")
		return echo.NewHTTPError(http.StatusBadRequest, "consent without challenge")
	}

	consent, response, err := h.hydraClient.GetConsentRequest(challenge)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to validate the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	} else if response.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"statusCode": response.StatusCode,
		}).Error("HTTP Response not OK. Failed to validate the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	}

	user, err := h.client.UserShow(consent.Subject)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"username": consent.Subject,
		}).Warn("Failed to find User record for consent")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	}

	if consent.Skip {
		log.WithFields(log.Fields{
			"user": consent.Subject,
			"oidc": consent.OidcContext,
		}).Info("Hydra requested we skip consent")

		// Check to make sure we have a valid user id
		_, err = h.client.UserShow(consent.Subject)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"username": consent.Subject,
			}).Warn("Failed to find User record for consent")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
		}

		completedRequest, response, err := h.hydraClient.AcceptConsentRequest(challenge, swagger.AcceptConsentRequest{
			GrantScope: consent.RequestedScope,
			Session: swagger.ConsentRequestSession{
				IdToken: map[string]interface{}{
					"uid":    string(user.Uid),
					"first":  string(user.First),
					"last":   string(user.Last),
					"groups": strings.Join(user.Groups, ";"),
					"email":  string(user.Email),
				},
			},
		})

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to accept the consent challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept consent")
		} else if response.StatusCode != http.StatusOK {
			log.WithFields(log.Fields{
				"statusCode": response.StatusCode,
			}).Error("HTTP response not OK. Failed to accept the consent challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept consent")
		}

		log.WithFields(log.Fields{
			"redirectURL": completedRequest.RedirectTo,
		}).Info("Consent challenge signed successfully")

		return c.Redirect(http.StatusFound, completedRequest.RedirectTo)
	}

	vars := map[string]interface{}{
		"csrf":      c.Get("csrf").(string),
		"consent":   consent,
		"challenge": challenge,
		"firstName": user.First,
	}

	return c.Render(http.StatusOK, "consent.html", vars)
}

func (h *Handler) ConsentPost(c echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to parse form")
		return echo.NewHTTPError(http.StatusBadRequest, "failed to parse form")
	}

	challenge := c.FormValue("challenge")
	if challenge == "" {
		log.WithFields(log.Fields{
			"ip": c.RealIP(),
		}).Error("Consent endpoint was called without a consent challenge")
		return echo.NewHTTPError(http.StatusBadRequest, "consent without challenge")
	}

	consent, response, err := h.hydraClient.GetConsentRequest(challenge)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to validate the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	} else if response.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"statusCode": response.StatusCode,
		}).Error("HTTP Response not OK. Failed to validate the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	}

	user, err := h.client.UserShow(consent.Subject)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"username": consent.Subject,
		}).Warn("Failed to find User record for consent")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate consent")
	}

	grantedScopes := c.Request().PostForm["scope"]
	if grantedScopes == nil {
		grantedScopes = []string{}
	}

	completedRequest, response, err := h.hydraClient.AcceptConsentRequest(challenge, swagger.AcceptConsentRequest{
		GrantScope:  grantedScopes,
		Remember:    true,  // TODO: make these configurable
		RememberFor: 86400, // TODO: make these configurable
		Session: swagger.ConsentRequestSession{
			IdToken: map[string]interface{}{
				"uid":    string(user.Uid),
				"first":  string(user.First),
				"last":   string(user.Last),
				"groups": strings.Join(user.Groups, ";"),
				"email":  string(user.Email),
			},
		},
	})

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to accept the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept consent")
	} else if response.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"statusCode": response.StatusCode,
		}).Error("HTTP response not OK. Failed to accept the consent challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept consent")
	}

	log.WithFields(log.Fields{
		"redirectURL": completedRequest.RedirectTo,
	}).Info("Consent challenge signed successfully")

	return c.Redirect(http.StatusFound, completedRequest.RedirectTo)
}

func (h *Handler) LoginOAuthGet(c echo.Context) error {
	// Get the challenge from the query.
	challenge := c.Request().URL.Query().Get("login_challenge")
	if challenge == "" {
		log.WithFields(log.Fields{
			"ip": c.RealIP(),
		}).Error("Login OAuth endpoint was called without a challenge")
		return echo.NewHTTPError(http.StatusBadRequest, "login without challenge")
	}

	login, response, err := h.hydraClient.GetLoginRequest(challenge)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to validate the login challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate login")
	} else if response.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"statusCode": response.StatusCode,
		}).Error("HTTP Response not OK. Failed to validate the login challenge")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate login")
	}

	if login.Skip {
		log.WithFields(log.Fields{
			"user": login.Subject,
			"oidc": login.OidcContext,
		}).Info("Hydra requested we skip login")

		// Check to make sure we have a valid user id
		_, err = h.client.UserShow(login.Subject)
		if err != nil {
			log.WithFields(log.Fields{
				"error":    err,
				"username": login.Subject,
			}).Warn("Failed to find User record for login")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to validate login")
		}

		completedRequest, response, err := h.hydraClient.AcceptLoginRequest(challenge, swagger.AcceptLoginRequest{
			Subject: login.Subject,
		})

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to accept the login challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept login")
		} else if response.StatusCode != http.StatusOK {
			log.WithFields(log.Fields{
				"statusCode": response.StatusCode,
			}).Error("HTTP Response not OK. Failed to accept the login challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to login consent")
		}

		log.WithFields(log.Fields{
			"redirectURL": completedRequest.RedirectTo,
		}).Info("Login challenge signed successfully")

		return c.Redirect(http.StatusFound, completedRequest.RedirectTo)
	}

	vars := map[string]interface{}{
		"csrf":      c.Get("csrf").(string),
		"challenge": challenge,
	}

	return c.Render(http.StatusOK, "login-oauth.html", vars)
}

func (h *Handler) LoginOAuthPost(c echo.Context) error {
	message := ""
	uid := c.FormValue("uid")
	password := c.FormValue("password")
	challenge := c.FormValue("challenge")

	_, err := h.tryAuth(uid, password)
	if err == nil {
		completedRequest, response, err := h.hydraClient.AcceptLoginRequest(challenge, swagger.AcceptLoginRequest{
			Subject:     uid,
			Remember:    true,  // TODO: make these configurable
			RememberFor: 86400, // TODO: make these configurable
		})

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to accept the login challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to accept login")
		} else if response.StatusCode != http.StatusOK {
			log.WithFields(log.Fields{
				"statusCode": response.StatusCode,
			}).Error("HTTP Response not OK. Failed to accept the login challenge")
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to login consent")
		}

		log.WithFields(log.Fields{
			"redirectURL": completedRequest.RedirectTo,
		}).Info("Login challenge signed successfully")

		return c.Redirect(http.StatusFound, completedRequest.RedirectTo)
	}

	message = err.Error()

	vars := map[string]interface{}{
		"csrf":      c.Get("csrf").(string),
		"challenge": challenge,
		"message":   message}

	return c.Render(http.StatusOK, "login-oauth.html", vars)
}

func (h *Handler) checkApiKey(c echo.Context) (*model.ApiKey, error) {
	apiKeyString := c.Get(ContextKeyApi)
	if apiKeyString == nil {
		return nil, nil
	}

	key, err := h.db.FetchApiKey(apiKeyString.(string))
	if err != nil {
		log.WithFields(log.Fields{
			"key":   apiKeyString,
			"error": err,
		}).Error("Failed to fetch api key")
		return nil, err
	}

	// Check api client is enabled in config
	if _, ok := h.apiClients[key.ClientID]; !ok {
		log.WithFields(log.Fields{
			"key":       key.Key,
			"user":      key.UserName,
			"client_id": key.ClientID,
		}).Error("Api client is not enabled in mokey.yaml")
		return nil, errors.New("Invalid api client")
	}

	err = h.db.RefreshApiKey(key)
	if err != nil {
		log.WithFields(log.Fields{
			"key":   apiKeyString,
			"error": err,
		}).Error("Failed to save api key")
		return nil, err
	}

	userRec, err := h.client.UserShow(key.UserName)
	if err != nil {
		log.WithFields(log.Fields{
			"key":   apiKeyString,
			"user":  key.UserName,
			"error": err,
		}).Error("Failed to fetch user from ipa")
		return nil, err
	}

	c.Set(ContextKeyUser, userRec)

	return key, nil
}
