# Installed in <SDK>/lib/cmake/OneOCR. Relocatable, with no build-machine paths.
get_filename_component(_ONEOCR_ROOT "${CMAKE_CURRENT_LIST_DIR}/../../.." ABSOLUTE)
if(NOT TARGET OneOCR::oneocr)
  add_library(OneOCR::oneocr SHARED IMPORTED)
  if(WIN32)
    set_target_properties(OneOCR::oneocr PROPERTIES
      IMPORTED_LOCATION "${_ONEOCR_ROOT}/bin/oneocr.dll"
      IMPORTED_IMPLIB "${_ONEOCR_ROOT}/lib/oneocr.lib")
  elseif(APPLE)
    set_target_properties(OneOCR::oneocr PROPERTIES
      IMPORTED_LOCATION "${_ONEOCR_ROOT}/lib/liboneocr.dylib")
  else()
    set_target_properties(OneOCR::oneocr PROPERTIES
      IMPORTED_LOCATION "${_ONEOCR_ROOT}/lib/liboneocr.so")
  endif()
  set_target_properties(OneOCR::oneocr PROPERTIES
    INTERFACE_INCLUDE_DIRECTORIES "${_ONEOCR_ROOT}/include"
    INTERFACE_COMPILE_FEATURES cxx_std_17
    ONEOCR_SDK_ROOT "${_ONEOCR_ROOT}")
  if(MSVC)
    set_property(TARGET OneOCR::oneocr APPEND PROPERTY INTERFACE_COMPILE_OPTIONS /utf-8)
  endif()
endif()
# Package the available SDK dependencies beside a consumer executable.
function(oneocr_copy_dependencies target)
  get_target_property(_oneocr_root OneOCR::oneocr ONEOCR_SDK_ROOT)
  add_custom_command(TARGET ${target} POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E make_directory "$<TARGET_FILE_DIR:${target}>/lib"
    VERBATIM)
  file(GLOB _oneocr_libraries "${_oneocr_root}/lib/*.dll" "${_oneocr_root}/lib/*.dylib"
    "${_oneocr_root}/lib/*.so*" "${_oneocr_root}/lib/runtime.json"
    "${_oneocr_root}/lib/LICENSE" "${_oneocr_root}/lib/*.txt")
  foreach(_oneocr_library IN LISTS _oneocr_libraries)
    add_custom_command(TARGET ${target} POST_BUILD
      COMMAND ${CMAKE_COMMAND} -E copy_if_different
        "${_oneocr_library}" "$<TARGET_FILE_DIR:${target}>/lib" VERBATIM)
  endforeach()
  if(WIN32)
    file(GLOB _oneocr_dependencies "${_oneocr_root}/bin/*.dll")
    foreach(_oneocr_dependency IN LISTS _oneocr_dependencies)
      add_custom_command(TARGET ${target} POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
          "${_oneocr_dependency}" "$<TARGET_FILE_DIR:${target}>" VERBATIM)
    endforeach()
  elseif(APPLE)
    set_property(TARGET ${target} APPEND PROPERTY BUILD_RPATH "@executable_path/lib")
  else()
    set_property(TARGET ${target} APPEND PROPERTY BUILD_RPATH "$ORIGIN/lib")
  endif()
  if(EXISTS "${_oneocr_root}/licenses")
    add_custom_command(TARGET ${target} POST_BUILD
      COMMAND ${CMAKE_COMMAND} -E copy_directory "${_oneocr_root}/licenses" "$<TARGET_FILE_DIR:${target}>/licenses" VERBATIM)
  endif()
  if(EXISTS "${_oneocr_root}/models/oneocr-cjk-en.ocrpack")
    add_custom_command(TARGET ${target} POST_BUILD
      COMMAND ${CMAKE_COMMAND} -E make_directory "$<TARGET_FILE_DIR:${target}>/models"
      COMMAND ${CMAKE_COMMAND} -E copy_if_different
        "${_oneocr_root}/models/oneocr-cjk-en.ocrpack" "$<TARGET_FILE_DIR:${target}>/models"
      VERBATIM)
  endif()
endfunction()
unset(_ONEOCR_ROOT)
