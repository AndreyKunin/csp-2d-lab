Feature: Starting a simulation
  Request validation and the start response contract.

  Background:
    Given the server is running with config "fast"

  @FT-START-01
  Scenario: A valid request starts a job
    Given a start request from fixture "D13-mixed-20"
    When I start the simulation
    Then the response status is 201
    And the job id is a 32-character hex string
    And the response job status is "running"
    And the response message is "Simulation started"
    And the response has RFC 3339 timestamps

  @FT-START-02
  Scenario: Two starts create two different jobs
    Given a start request from fixture "D13-mixed-20"
    When I start the simulation
    And I start the simulation
    Then the two started jobs have different ids

  @FT-START-03
  Scenario: Explicit parameters are applied
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "eliteCount" is 4
    And the request parameter "mutationRate" is 0.05
    And the request parameter "crossoverRate" is 0.5
    And the request parameter "maxEpochsCount" is 77
    And the request parameter "windowSize" is 7
    When I start the simulation
    Then the response status is 201
    And the reported parameter "eliteCount" is 4
    And the reported parameter "mutationRate" is 0.05
    And the reported parameter "crossoverRate" is 0.5
    And the reported parameter "maxEpochsCount" is 77
    And the reported parameter "windowSize" is 7

  # The "fast" profile defaults: elite 5, window 10, 200 epochs, population 20..60.
  @FT-START-04
  Scenario: Partial parameters are completed from the defaults
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "mutationRate" is 0.03
    When I start the simulation
    Then the response status is 201
    And the reported parameter "mutationRate" is 0.03
    And the reported parameter "eliteCount" is 5
    And the reported parameter "crossoverRate" is 0.75
    And the reported parameter "windowSize" is 10
    And the reported parameter "maxEpochsCount" is 200
    And the reported parameter "populationSize" is 60

  # D13 has n = 20 parts, so n squared = 400.
  @FT-START-05
  Scenario Outline: Population size is max(min, min(n squared, max))
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "populationSize.min" is <min>
    And the request parameter "populationSize.max" is <max>
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is <expected>

    Examples:
      | min | max | expected |
      | 500 | 900 | 500      |
      | 30  | 900 | 400      |
      | 30  | 50  | 50       |

  @FT-START-06
  Scenario: Malformed JSON is rejected
    Given a start request from fixture "X09-malformed"
    When I start the simulation
    Then the response status is 400
    And the server is healthy

  # BE-02: Request.Validate, before anything is built.
  @FT-START-07
  Scenario Outline: Invalid inputs are rejected with 400
    Given a start request from fixture "<fixture>"
    When I start the simulation
    Then the response status is 400
    And the server is healthy

    Examples:
      | fixture            |
      | X01-no-parts       |
      | X02-zero-count     |
      | X03-negative-count |
      | X04-zero-dimension |
      | X05-negative-width |
      | X07-negative-kerf  |
      | X08-zero-sheet     |

  @FT-START-08
  Scenario: A non-numeric label is a client error
    Given a start request from fixture "X06-non-numeric-label"
    When I start the simulation
    Then the response status is 400

  # Ten million parts: refused before anything is allocated (the total part
  # count is limited by inputs.maxTotalCount in parameters.json). Runs on its
  # own server, as it once exhausted the memory.
  @FT-START-09
  Scenario: An oversized part count is refused
    Given a server started with config "fast"
    And a start request from fixture "X10-oversized-count"
    When I start the simulation
    Then the response arrives within 10 seconds
    And the response status is one of 400, 413
    And the server is healthy

  @FT-START-10
  Scenario: A part larger than the sheet is reported
    Given a start request from fixture "D10-too-big"
    When I start the simulation
    Then the response status is 201
    When I wait for job completion within 20 seconds
    Then the job status is "error"

  # Small inputs (GA-05): for n parts in [4, 10] the population minimum is n²/2,
  # so the population is n², and the elite count is 1 (n = 4) or 2; the maximum
  # stays as configured; above 10 parts nothing is overridden. The overrides win
  # over request values.
  @FT-START-11
  Scenario: Four parts get population 16 and one elite
    Given a start request from fixture "D04-four-identical"
    And the request parameter "populationSize.min" is 500
    And the request parameter "populationSize.max" is 60000
    And the request parameter "eliteCount" is 7
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is 16
    And the reported parameter "eliteCount" is 1

  @FT-START-12
  Scenario: Five parts get population 25 and two elites
    Given a start request from fixture "D07-identical-multi"
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is 25
    And the reported parameter "eliteCount" is 2

  @FT-START-13
  Scenario: Twenty parts keep the configured population and the requested elite count
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "eliteCount" is 7
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is 60
    And the reported parameter "eliteCount" is 7

  @FT-START-14
  Scenario: A small input keeps the requested population maximum
    Given a start request from fixture "D04-four-identical"
    And the request parameter "populationSize.min" is 3
    And the request parameter "populationSize.max" is 10
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is 10
    And the reported parameter "eliteCount" is 1
