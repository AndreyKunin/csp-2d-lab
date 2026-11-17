Feature: Service endpoints
  Basic availability and contract of the non-simulation endpoints.

  Background:
    Given the server is running with config "fast"

  @FT-SVC-01
  Scenario: Heartbeat answers
    When I send a "GET" request to "/heartbeat"
    Then the response status is 200
    And the response body is "{}"

  @FT-SVC-02
  Scenario: Default parameters reflect the loaded configuration
    When I request the default parameters
    Then the response status is 200
    And the default parameters match the server config

  @FT-SVC-03
  Scenario: API responses are JSON and not cacheable
    When I request the default parameters
    Then the response header "Cache-Control" is "no-cache, no-store, must-revalidate"
    And the response header "Content-Type" contains "application/json"

  @FT-SVC-04
  Scenario: The frontend is served at the root
    When I send a "GET" request to "/"
    Then the response status is 200
    And the response body contains "<html"

  @FT-SVC-05
  Scenario: Unknown API route
    When I send a "GET" request to "/api/v1/nope"
    Then the response status is 404

  # Currently 404: the static catch-all route answers before the method check.
  @FT-SVC-06 @pending
  Scenario: Wrong HTTP method on a simulation route
    When I send a "GET" request to "/api/v1/simulation/start"
    Then the response status is 405
