// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

package io.github.sonicboom15.daimon;

/**
 * Unchecked exception thrown by the Daimon client on HTTP errors or
 * protocol-level error chunks from the sidecar.
 */
public class DaimonException extends RuntimeException {

    public DaimonException(String message) {
        super(message);
    }

    public DaimonException(String message, Throwable cause) {
        super(message, cause);
    }
}
