Feature: Boundary simulation parameters
  Boundary values for the start parameters.

  Background:
    Given the server is running with config "fast"

  @FT-PAR-01
  Scenario: One elite
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | eliteCount     | 1  |
      | maxEpochsCount | 50 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And all statistics are finite
    And every solution is valid

  @FT-PAR-02
  Scenario: Zero elites are rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "eliteCount" is 0
    When I start the simulation
    Then the response status is 400

  @FT-PAR-03
  Scenario: Elite count equal to the population size
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "populationSize.min" is 20
    And the request parameter "populationSize.max" is 20
    And the request parameter "eliteCount" is 20
    And the request parameter "maxEpochsCount" is 30
    When I start the simulation
    Then the start is rejected or the job completes without error within 60 seconds

  @FT-PAR-04
  Scenario: Elite count above the population size is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "populationSize.min" is 20
    And the request parameter "populationSize.max" is 20
    And the request parameter "eliteCount" is 21
    When I start the simulation
    Then the response status is 400

  @FT-PAR-05
  Scenario: The smallest population
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | populationSize.min | 3  |
      | populationSize.max | 3  |
      | eliteCount         | 1  |
      | maxEpochsCount     | 50 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the reported parameter "populationSize" is 3
    And every solution is valid

  # No rule relates the two bounds: the minimum wins (n^2 clamped to [min, max],
  # BuildParameters). D13 has 20 parts: n^2 = 400, max 50, min 100 -> 100.
  @FT-PAR-06
  Scenario: A minimum population above the maximum wins
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "populationSize.min" is 100
    And the request parameter "populationSize.max" is 50
    When I start the simulation
    Then the response status is 201
    And the reported parameter "populationSize" is 100

  @FT-PAR-07
  Scenario: Mutation rate 0 either runs or is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "mutationRate" is 0
    And the request parameter "maxEpochsCount" is 30
    When I start the simulation
    Then the start is rejected or the job completes without error within 60 seconds

  # The rule is 0 <= mutationRate <= 1 (config/parameters.json).
  @FT-PAR-07
  Scenario Outline: Mutation rates outside [0, 1] are rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "mutationRate" is <rate>
    When I start the simulation
    Then the response status is 400

    Examples:
      | rate |
      | 1.5  |
      | -0.1 |

  # The epoch limit is at least 1 (config/parameters.json).
  @FT-PAR-08
  Scenario: Zero epochs are rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "maxEpochsCount" is 0
    When I start the simulation
    Then the response status is 400
    And the error message contains "maxEpochsCount must be a whole number, at least 1"

  @FT-PAR-09
  Scenario: One epoch
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 1
    When I wait for job completion within 30 seconds
    Then the job status is "completed"
    And the epoch number is exactly 1
    And every solution is valid

  @FT-PAR-10
  Scenario: A window size of 0 is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "windowSize" is 0
    When I start the simulation
    Then the response status is 400

  @FT-PAR-11
  Scenario: A window longer than the run never stops it early
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 250  |
      | maxEpochsCount | 60   |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is exactly 60

  @FT-PAR-12
  Scenario: An odd number of offspring slots
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | populationSize.min | 21 |
      | populationSize.max | 21 |
      | eliteCount         | 4  |
      | maxEpochsCount     | 50 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  # The window size is at most 250 (config/parameters.json). A huge window
  # once pre-allocated 800 MB for 10^8. The fast profile allows larger windows
  # for its one-cycle scenarios, so these run on the production rules.
  @FT-PAR-13
  Scenario Outline: A window size above 250 is rejected
    Given a server started with config "defaults"
    And a start request from fixture "D13-mixed-20"
    And the request parameter "windowSize" is <size>
    When I start the simulation
    Then the response status is 400
    And the error message contains "windowSize must be a whole number from 1 to 250"

    Examples:
      | size      |
      | 251       |
      | 100000000 |

  @FT-PAR-14
  Scenario: The largest window size is accepted
    Given a server started with config "defaults"
    And a start request from fixture "D13-mixed-20"
    And the request parameter "windowSize" is 250
    When I start the simulation
    Then the response status is 201
    And the reported parameter "windowSize" is 250
