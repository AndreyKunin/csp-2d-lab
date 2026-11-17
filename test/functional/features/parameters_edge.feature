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

  @FT-PAR-02 @pending
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

  @FT-PAR-04 @pending
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

  @FT-PAR-06 @pending
  Scenario: A minimum population above the maximum is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "populationSize.min" is 100
    And the request parameter "populationSize.max" is 50
    When I start the simulation
    Then the response status is 400

  @FT-PAR-07
  Scenario: Mutation rate 0 either runs or is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "mutationRate" is 0
    And the request parameter "maxEpochsCount" is 30
    When I start the simulation
    Then the start is rejected or the job completes without error within 60 seconds

  @FT-PAR-07 @pending
  Scenario Outline: Mutation rates outside (0, 1) are rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "mutationRate" is <rate>
    When I start the simulation
    Then the response status is 400

    Examples:
      | rate |
      | 1    |
      | 1.5  |
      | -0.1 |

  @FT-PAR-08
  Scenario: Zero epochs
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 0
    When I wait for job completion within 30 seconds
    Then the job status is "completed"
    And the result is empty or valid
    And the server is healthy

  @FT-PAR-09
  Scenario: One epoch
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 1
    When I wait for job completion within 30 seconds
    Then the job status is "completed"
    And the epoch number is exactly 1
    And every solution is valid

  @FT-PAR-10 @pending
  Scenario: A window size of 0 is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "windowSize" is 0
    When I start the simulation
    Then the response status is 400

  @FT-PAR-11
  Scenario: A window longer than the run never stops it early
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 1000 |
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

  # The metric window takes memory for the epochs a job has run, never for the
  # client-supplied windowSize (pre-allocating it reserved 800 MB for 10^8).
  # A long window is legitimate: it also keeps the early stop off. Runs on its
  # own server.
  @FT-PAR-13
  Scenario: A huge window size reserves no memory
    Given a server started with config "fast"
    And I record the server memory baseline
    And a start request from fixture "D13-mixed-20"
    And the request parameter "windowSize" is 100000000
    When I start the simulation
    Then the response status is 201
    And the epoch number increases within 10 seconds
    And the server memory grew by less than 50 MB
