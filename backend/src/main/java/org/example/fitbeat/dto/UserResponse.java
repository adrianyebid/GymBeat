package org.example.fitbeat.dto;

public record UserResponse(
        Long id,
        String firstName,
        String lastName,
        String email
) {
}
