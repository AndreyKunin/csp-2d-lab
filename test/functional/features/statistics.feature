Feature: Status statistics
  Status payload schema and metric sanity.

  Background:
    Given the server is running with config "fast"

  @FT-STAT-01 @FT-STAT-02 @FT-STAT-03 @FT-STAT-04
  Scenario: Statistics are complete, finite and consistent during a run
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I poll the status for 2 seconds
    Then the status payload has the full statistics schema
    And all statistics are finite
    And the statistics are internally consistent
    And the epoch numbers never decrease

  @FT-STAT-05
  Scenario: The best fitness vector matches the scalar best fitness
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 60
    When I wait for job completion within 60 seconds
    Then the best fitness vector is consistent with the sheet area

  @FT-STAT-06
  Scenario: The selected species are the sorted elites
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | eliteCount     | 5         |
      | maxEpochsCount | 100000000 |
      | windowSize     | 250       |
      | cycleCount     | 1000000   |
    And the epoch number increases within 10 seconds
    When I pause the simulation
    Then the selected species are sorted and number 5

  # D04 has a tiny gene space, so offspring collide with the elites often.
  @FT-STAT-07
  Scenario: The saturation rate is a rate
    Given a long-running simulation for fixture "D04-four-identical"
    And the epoch number increases within 10 seconds
    When I poll the status for 2 seconds
    Then the saturation rate stays within 0 and 1

  # Within one cycle: after a restart the window still holds the previous
  # cycle's better epochs. A window far above the production limit of 250
  # (allowed by the fast profile) keeps the run in its first cycle.
  @FT-STAT-08
  Scenario: Window averages are never below the best fitness
    Given a running simulation for fixture "D13-mixed-20" with parameters:
      | windowSize     | 1000000   |
      | maxEpochsCount | 100000000 |
    And the epoch number increases within 10 seconds
    When I poll the status for 2 seconds
    Then the window averages are not below the best fitness

  @FT-STAT-09
  Scenario: Status right after the start
    Given a start request from fixture "D15-large-150"
    When I start the simulation
    And I fetch the status
    Then the response status is 200
