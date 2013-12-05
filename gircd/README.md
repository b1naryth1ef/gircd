

# loops
- Accept Loop, accepts connections and creates clients
- Read Loop, iterates over clients and non-blockingly reads to a Queue
- Parse Loop (multiplexable), iterates over clients and reads from their queue, locks them, and updates accordingly
- Update Looop (multiplexable), iterates over clients and checks for waiting updates, checks if they are valid, and updates (also checks ping)