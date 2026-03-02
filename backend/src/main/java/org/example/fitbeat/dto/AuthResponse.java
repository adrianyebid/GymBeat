package org.example.fitbeat.dto;

public record AuthResponse(
        String message,
        UserResponse user
) {
}
