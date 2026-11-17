Feature: Concurrent clients
  Many clients and jobs at once. FT-CONC-06 (no data race
  reports) is the whole suite run with a race-built server:
  `make test-functional-race`.

  Background:
    Given the server is running with config "fast"

  @FT-CONC-01
  Scenario: Several jobs with different inputs run side by side
    When I start these simulations:
      | D04-four-identical | 60 |
      | D13-mixed-20       | 60 |
      | D17-many-labels    | 60 |
      | D11-unconstrained  | 60 |
    Then all started simulations complete within 120 seconds
    And every started simulation has a valid result for its own input

  @FT-CONC-02
  Scenario: Many clients poll one running job
    Given a long-running simulation for fixture "D13-mixed-20"
    When 10 clients poll the status at 20 Hz for 3 seconds
    Then every poll succeeded
    And the server is healthy

  @FT-CONC-03
  Scenario: Two simultaneous pause requests
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I run concurrently:
      | pause |
      | pause |
    Then every concurrent call answered within 5 seconds
    And the job status is "paused"

  @FT-CONC-04
  Scenario Outline: Conflicting simultaneous calls
    Given a long-running simulation for fixture "D13-mixed-20"
    And the epoch number increases within 10 seconds
    When I run concurrently:
      | <first>  |
      | <second> |
    Then every concurrent call answered within 5 seconds
    And the server is healthy

    Examples:
      | first     | second |
      | pause     | delete |
      | delete    | status |
      | configure | pause  |
      | result    | pause  |

  @FT-CONC-05
  Scenario: Start/delete churn
    When I start and delete 50 simulations for fixture "D13-mixed-20" with a 200 ms pause
    Then every repeated call succeeded
    And the server is healthy
