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
    INTERFACE_COMPILE_FEATURES cxx_std_17)
endif()
unset(_ONEOCR_ROOT)
