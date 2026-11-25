Feature: Simulation results
  The result endpoint returns valid, ordered solutions.

  Background:
    Given the server is running with config "fast"

  @FT-RES-01 @FT-RES-02 @FT-RES-03
  Scenario: Result of a completed simulation
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | eliteCount     | 5  |
      | maxEpochsCount | 40 |
    When I wait for job completion within 60 seconds
    And I fetch the result
    Then the response status is 200
    And every solution is valid
    And the result has between 1 and 5 solutions
    And the solutions are sorted by fitness

  @FT-RES-04
  Scenario: Result of a paused simulation
    Given a long-running simulation for fixture "D17-many-labels"
    And the epoch number increases within 10 seconds
    When I pause the simulation
    And I fetch the result
    Then every solution is valid

  @FT-RES-05
  Scenario: Results fetched while the simulation runs are always valid
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I fetch and validate the result 50 times
    Then the server is healthy

  # A large population makes the initial population take a while to build.
  @FT-RES-06
  Scenario: Result requested before the first epoch
    Given a running simulation for fixture "D15-large-150" with parameters:
      | populationSize.min | 3000 |
      | populationSize.max | 3000 |
    When I fetch the result without waiting
    Then the response status is one of 200, 409
    And the server is healthy

  @FT-RES-07 @FT-RES-08
  Scenario: Results use only requested labels and contiguous sheet numbers
    Given a running simulation for fixture "D17-many-labels" with maxEpochsCount 40
    When I wait for job completion within 60 seconds
    And I fetch the result
    Then the solutions only use labels from the request
    And the sheet numbers are contiguous from 0

  # Within one cycle: a new cycle starts over from random solutions, and the
  # result is the current cycle's best. A window far above the production limit
  # of 250 (allowed by the fast profile) keeps the run in its first cycle.
  @FT-RES-09
  Scenario: The best result never gets worse
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 1000000   |
      | maxEpochsCount | 100000000 |
    And the epoch number increases within 10 seconds
    And I pause the simulation
    And I remember the best result fitness
    When I resume the simulation
    And the epoch number increases within 5 seconds
    And I pause the simulation
    Then the best result fitness is not worse than remembered
