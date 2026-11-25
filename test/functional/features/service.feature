Feature: Service endpoints
  Basic availability and contract of the non-simulation endpoints.

  Background:
    Given the server is running with config "fast"

  @FT-SVC-01
  Scenario: The health check answers
    When I send a "GET" request to "/api/service/v1/healthz"
    Then the response status is 200
    And the response body is "{}"

  @FT-SVC-02
  Scenario: Parameter definitions reflect the loaded configuration
    When I request the parameter definitions
    Then the response status is 200
    And the parameter definitions match the server config

  # The UI shows the server's defaults and validates with the server's rules
  # (application.yaml); the fast profile allows windows up to 10^6.
  @FT-SVC-07
  Scenario: Parameter rules are the server's
    When I request the parameter definitions
    Then the response status is 200
    And the parameter rule of "windowSize" is "int[1-1000000]"
    And the parameter rule of "populationSize.min" is "int[3-)"
    And the parameter rule of "crossoverRate" is "float(0-1]"
    And the response has 12 parameter definitions

  @FT-SVC-03
  Scenario: API responses are JSON and not cacheable
    When I request the parameter definitions
    Then the response header "Cache-Control" is "no-cache, no-store, must-revalidate"
    And the response header "Content-Type" contains "application/json"

  # webserver.pprof is off unless a profile enables it (BE-06; UT-WEB-01 checks
  # the handlers with the flag on).
  @FT-SVC-08
  Scenario: The profiler is not served by default
    When I send a "GET" request to "/debug/pprof/"
    Then the response status is 404

  @FT-SVC-04
  Scenario: The frontend is served at the root
    When I send a "GET" request to "/"
    Then the response status is 200
    And the response body contains "<html"

  @FT-SVC-05
  Scenario Outline: Unknown API route
    When I send a "<method>" request to "<path>"
    Then the response status is 404

    Examples:
      | method | path                         |
      | GET    | /api/v1/nope                 |
      | POST   | /api/v1/simulation/start     |
      | POST   | /api/v1/pdf/generate         |
      | GET    | /heartbeat                   |
      | GET    | /api/service/healthz         |
      | GET    | /api/v1/simulation/0123abcd  |

  # BE-09: the route groups' catch-alls answered 404. The Allow header lists
  # the methods whose routes match the path.
  @FT-SVC-06
  Scenario Outline: Wrong HTTP method on an API route
    When I send a "<method>" request to "<path>"
    Then the response status is 405
    And the response header "Allow" is "<allow>"

    Examples:
      | method | path                                  | allow             |
      | POST   | /api/v1/simulation                    | PUT               |
      | PUT    | /api/v1/simulation/0123abcd           | GET, POST, DELETE |
      | GET    | /api/v1/report                        | POST              |
      | POST   | /api/service/v1/parameter-definitions | GET               |
