Feature: Cycles and championship
  A cycle ends when the best fitness has not improved for windowSize epochs:
  its elite is kept and the population starts over from random individuals.
  After cycleCount cycles all kept elites plus random individuals compete in a
  championship; when that stagnates too, the job ends. cycleCount 0 ends the
  job at the first stagnation.

  Background:
    Given the server is running with config "fast"

  # The "fast" profile defaults: cycleCount 10, window 10.
  @FT-CYC-01
  Scenario: The cycle count comes from the configuration
    Given a start request from fixture "D13-mixed-20"
    When I start the simulation
    Then the response status is 201
    And the reported parameter "cycleCount" is 10

  @FT-CYC-02
  Scenario: A start request sets the cycle count
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "cycleCount" is 3
    When I start the simulation
    Then the response status is 201
    And the reported parameter "cycleCount" is 3

  @FT-CYC-03
  Scenario: The job ends when the championship stagnates
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | cycleCount     | 1         |
      | windowSize     | 5         |
      | maxEpochsCount | 100000000 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the run finished 1 cycle
    And the championship has started
    And the epoch number is at most 100000

  # The cycle count is at least 1 (config/parameters.json).
  @FT-CYC-04
  Scenario: Cycle count 0 is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "cycleCount" is 0
    When I start the simulation
    Then the response status is 400
    And the error message contains "cycleCount must be a whole number, at least 1"

  @FT-CYC-05
  Scenario: A job that never stagnates runs to the epoch limit
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 250     |
      | cycleCount     | 1000000 |
      | maxEpochsCount | 20      |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is exactly 20
    And the run finished 0 cycles
    And the championship has not started

  @FT-CYC-06
  Scenario: The cycle count can be reconfigured on a paused job
    Given a long-running simulation for fixture "D13-mixed-20"
    And I pause the simulation
    When I configure the simulation with:
      | cycleCount     | 2         |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
    Then the response status is 200
    And the job status is "paused"
    And the reported parameter "cycleCount" is 2

  @FT-CYC-07
  Scenario: A cycle count that is not a whole number is rejected
    Given a long-running simulation for fixture "D13-mixed-20"
    When I configure the simulation with raw body '{"parameters": {"cycleCount": 1.5}}'
    Then the response status is 400
    And the job status is "running"

  @FT-CYC-08
  Scenario: A negative cycle count is rejected
    Given a start request from fixture "D13-mixed-20"
    And the request parameter "cycleCount" is -1
    When I start the simulation
    Then the response status is 400
